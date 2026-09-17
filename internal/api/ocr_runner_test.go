package api

import "testing"

func TestExtractRunnerRawText(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			// 真实 runner 在没有检测到文本框时的输出：不能把启动横幅当成识别文字
			name: "无文本框时不返回启动横幅",
			output: `modelsPath=ocr/models
model det path=ocr/models/license_det.onnx
=====Init Models=====
--- Init DbNet ---
--- Init AngleNet ---
--- Init CrnnNet ---
total keys size(6625)
Init Models Success!
=====Start detect=====
ScaleParam(sw:740,sh:420,dw:736,dh:416,0.994595,0.990476)
---------- step: dbNet getTextBoxes ----------
dbNetTime(197.741643ms)
=====End detect=====
FullDetectTime(197.755308ms)`,
			want: "",
		},
		{
			name: "End detect 之后的文字按行收集",
			output: `Init Models Success!
=====Start detect=====
=====End detect=====
粤A12345
2026-05-14 10:22:00
FullDetectTime(120ms)`,
			want: "粤A12345\n2026-05-14 10:22:00",
		},
		{
			name: "纯文本输出回退到最后一行有效文字",
			output: `Init Models Success!
some banner
粤B88888`,
			want: "粤B88888",
		},
		{
			name:   "全是日志行时返回空",
			output: "Init Models Success!\nFullDetectTime(120ms)",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractRunnerRawText(tt.output); got != tt.want {
				t.Fatalf("extractRunnerRawText() = %q，期望 %q", got, tt.want)
			}
		})
	}
}

func TestIsRunnerLogLine(t *testing.T) {
	logLines := []string{
		"modelsPath=ocr/models",
		"=====End detect=====",
		"--- Init DbNet ---",
		"total keys size(6625)",
		"Init Models Success!",
		"Init Models Failed!",
		"FullDetectTime(197.755308ms)",
		"ScaleParam(sw:740,sh:420)",
		"numThread(2),padding(50)",
	}
	for _, line := range logLines {
		if !isRunnerLogLine(line) {
			t.Errorf("应被识别为日志行: %q", line)
		}
	}

	textLines := []string{"粤A12345", "2026-05-14 10:22:00", "某某停车场"}
	for _, line := range textLines {
		if isRunnerLogLine(line) {
			t.Errorf("不应被识别为日志行: %q", line)
		}
	}
}
