package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dezhishen/parking-violation-assistant/internal/api"
	"github.com/dezhishen/parking-violation-assistant/internal/db"
)

//go:embed frontend/dist
var frontendFS embed.FS

// noBrowser 启动时不自动打开浏览器（开发模式由 Vite 提供页面）
var noBrowser = flag.Bool("no-browser", false, "启动时不自动打开浏览器")

func main() {
	port := resolvePort()

	// 数据目录：放在可执行文件同级目录的 data/ 下
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal("获取可执行文件路径失败:", err)
	}
	exeDir := filepath.Dir(exePath)
	dataDir := filepath.Join(exeDir, "data")
	uploadDir := filepath.Join(dataDir, "uploads")

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatal("创建上传目录失败:", err)
	}

	// 初始化数据库
	if err := db.Init(dataDir); err != nil {
		log.Fatal("数据库初始化失败:", err)
	}
	defer db.Close()

	// 设置上传目录
	api.SetUploadDir(uploadDir)

	listenAddr := "127.0.0.1:0"
	if port > 0 {
		listenAddr = fmt.Sprintf("127.0.0.1:%d", port)
	}

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal("监听端口失败:", err)
	}
	defer listener.Close()

	actualPort := listener.Addr().(*net.TCPAddr).Port
	addr := fmt.Sprintf("127.0.0.1:%d", actualPort)

	// 注册路由
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	// 提供前端静态文件
	distFS, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatal("加载前端文件失败:", err)
	}
	fileServer := http.FileServer(http.FS(distFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		// 文本类静态资源走 gzip + 长缓存
		if served := serveCompressed(w, r, distFS, name); served {
			return
		}

		f, err := distFS.Open(name)
		if err != nil {
			// 静态资源（带扩展名）缺失时返回 404，避免把 index.html 当成 JS/CSS 返回。
			if filepath.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
			// 其余路径视为前端路由，回退到 index.html（SPA 路由）。
			r.URL.Path = "/"
		} else {
			info, statErr := f.Stat()
			f.Close()
			// 不暴露内嵌产物的目录列表
			if statErr == nil && info.IsDir() {
				http.NotFound(w, r)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	})

	log.Printf("启动服务: http://%s", addr)

	if !*noBrowser {
		// 延迟打开浏览器
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowser(fmt.Sprintf("http://%s", addr))
		}()
	}

	// 不设置 WriteTimeout：导出 Excel 等长任务需要较长时间。
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(server.Serve(listener))
}

func resolvePort() int {
	flagPort := flag.Int("port", 0, "固定监听端口（不传则随机可用端口）")
	flag.Parse()

	if *flagPort != 0 {
		if *flagPort < 1 || *flagPort > 65535 {
			log.Fatalf("非法端口: %d", *flagPort)
		}
		return *flagPort
	}

	envPort := os.Getenv("COUNTCAR_PORT")
	if envPort == "" {
		return 0
	}

	p, err := strconv.Atoi(envPort)
	if err != nil || p < 1 || p > 65535 {
		log.Fatalf("环境变量 COUNTCAR_PORT 非法: %s", envPort)
	}
	return p
}

// openBrowser 在默认浏览器中打开 URL
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("打开浏览器失败: %v", err)
	}
}
