package api

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestExtractPlateNumber(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"带时间的识别结果", "京A12345 2026-05-14 10:22:00", "京A12345"},
		{"新能源号牌", "粤B12345D", "粤B12345D"},
		{"小写自动转大写", "粤a12345", "粤A12345"},
		{"同一行内插入空格", "粤 A 12345", "粤A12345"},
		{"多行文本中提取", "某某停车场\n粤C88888\n2026-05-14", "粤C88888"},
		{"号牌下一行是日期时不粘连数字", "京A12345\n2026-05-14 10:22:00", "京A12345"},
		{"号牌后紧跟中文", "粤D66666号车", "粤D66666"},
		{"号牌被换行拆开时不识别", "粤 A \n 12345", ""},
		{"无车牌", "没有车牌号", ""},
		{"缺少省份简称不识别", "A12345", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractPlateNumber(tt.text); got != tt.want {
				t.Fatalf("extractPlateNumber(%q) = %q，期望 %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestExtractParkingTime(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"标准格式", "粤A12345 2026-05-14 10:22:00", "2026-05-14 10:22:00"},
		{"中文年月日", "2026年5月14日 10:22", "2026-05-14 10:22:00"},
		{"斜杠与时分", "2026/05/14 10时22分30", "2026-05-14 10:22:30"},
		{"无秒", "2026-05-14 08:05", "2026-05-14 08:05:00"},
		{"非法日期不返回", "2026-02-30 10:00", ""},
		{"非法小时不返回", "2026-05-14 25:00", ""},
		{"无时间", "粤A12345", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractParkingTime(tt.text); got != tt.want {
				t.Fatalf("extractParkingTime(%q) = %q，期望 %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestVerifyImageContent(t *testing.T) {
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	gifHeader := []byte("GIF89a")
	bmpHeader := []byte{0x42, 0x4D}

	tests := []struct {
		name    string
		content []byte
		wantErr bool
	}{
		{"PNG", append(pngHeader, bytes.Repeat([]byte{0}, 32)...), false},
		{"JPEG", append(jpegHeader, bytes.Repeat([]byte{0}, 32)...), false},
		{"GIF", append(gifHeader, bytes.Repeat([]byte{0}, 32)...), false},
		{"BMP", append(bmpHeader, bytes.Repeat([]byte{0}, 32)...), false},
		{"伪装成图片的文本", []byte("this is not an image at all"), true},
		{"HTML", []byte("<!doctype html><html></html>"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := verifyImageContent(bytes.NewReader(tt.content))
			if tt.wantErr && err == nil {
				t.Fatalf("期望校验失败，实际通过")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("期望校验通过，实际报错: %v", err)
			}
		})
	}
}

func TestVerifyImageContentRewinds(t *testing.T) {
	content := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, []byte("payload")...)
	r := bytes.NewReader(content)
	if err := verifyImageContent(r); err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	rest, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if !bytes.Equal(rest, content) {
		t.Fatalf("校验后读取位置未复位，得到 %q", rest)
	}
}

func TestNewUploadName(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		name, err := newUploadName(".png")
		if err != nil {
			t.Fatalf("生成文件名失败: %v", err)
		}
		if !strings.HasSuffix(name, ".png") {
			t.Fatalf("扩展名不正确: %s", name)
		}
		if seen[name] {
			t.Fatalf("文件名重复: %s", name)
		}
		seen[name] = true
	}
}

func TestResolveImageFilePath(t *testing.T) {
	uploadDir = "/tmp/pva-uploads"
	defer func() { uploadDir = "" }()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"uploads 前缀", "/uploads/a.png", "/tmp/pva-uploads/a.png", false},
		{"相对 uploads", "uploads/b.png", "/tmp/pva-uploads/b.png", false},
		{"历史裸文件名", "c.png", "/tmp/pva-uploads/c.png", false},
		{"完整 URL", "http://127.0.0.1:8080/uploads/d.png", "/tmp/pva-uploads/d.png", false},
		{"上传目录内的绝对路径", "/tmp/pva-uploads/f.png", "/tmp/pva-uploads/f.png", false},
		{"穿越路径被压回上传目录", "/uploads/../e.png", "/tmp/pva-uploads/e.png", false},
		{"穿越到上级目录被拒绝", "/uploads/..", "", true},
		{"上传目录外的绝对路径被拒绝", "/etc/passwd", "", true},
		{"空路径", "   ", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveImageFilePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际得到 %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveImageFilePath(%q) = %q，期望 %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsInsideDir(t *testing.T) {
	tests := []struct {
		dir  string
		path string
		want bool
	}{
		{"/data/uploads", "/data/uploads/a.png", true},
		{"/data/uploads", "/data/uploads/sub/a.png", true},
		{"/data/uploads", "/data/records.db", false},
		{"/data/uploads", "/etc/passwd", false},
	}

	for _, tt := range tests {
		if got := isInsideDir(tt.dir, tt.path); got != tt.want {
			t.Fatalf("isInsideDir(%q, %q) = %v，期望 %v", tt.dir, tt.path, got, tt.want)
		}
	}
}
