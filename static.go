package main

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
)

// compressibleExt 需要 gzip 压缩的静态资源扩展名
var compressibleExt = map[string]bool{
	".js":   true,
	".mjs":  true,
	".css":  true,
	".html": true,
	".svg":  true,
	".json": true,
	".txt":  true,
	".map":  true,
}

// mimeByExt 压缩响应需要显式给出的内容类型
var mimeByExt = map[string]string{
	".js":   "text/javascript; charset=utf-8",
	".mjs":  "text/javascript; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".html": "text/html; charset=utf-8",
	".svg":  "image/svg+xml",
	".json": "application/json; charset=utf-8",
	".txt":  "text/plain; charset=utf-8",
	".map":  "application/json; charset=utf-8",
}

// gzipCache 缓存已压缩的静态资源（内嵌资源不可变，压缩一次即可长期复用）
var gzipCache sync.Map // string -> []byte

// serveCompressed 尝试以内嵌 gzip 的形式返回静态资源。
// 返回 false 表示未处理，调用方应交给默认文件服务。
func serveCompressed(w http.ResponseWriter, r *http.Request, distFS fs.FS, name string) bool {
	ext := strings.ToLower(path.Ext(name))
	if !compressibleExt[ext] {
		return false
	}
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}

	raw, err := fs.ReadFile(distFS, name)
	if err != nil {
		return false
	}

	payload, ok := gzipCache.Load(name)
	if !ok {
		var buf bytes.Buffer
		zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
		if err != nil {
			return false
		}
		if _, err := zw.Write(raw); err != nil {
			_ = zw.Close()
			return false
		}
		if err := zw.Close(); err != nil {
			return false
		}
		compressed := buf.Bytes()
		gzipCache.Store(name, compressed)
		payload = compressed
	}

	header := w.Header()
	header.Set("Content-Encoding", "gzip")
	header.Add("Vary", "Accept-Encoding")
	if ct, ok := mimeByExt[ext]; ok {
		header.Set("Content-Type", ct)
	}
	if name == "index.html" {
		// 入口文件会随构建变化，不能长缓存
		header.Set("Cache-Control", "no-cache")
	} else {
		// 其余资源名带构建 hash，内容不可变
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	}

	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(payload.([]byte))
	}
	return true
}
