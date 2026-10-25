package app

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCompressPromptLossless(t *testing.T) {
	rawToolJSON := "Available tools:\n[\n  {\n    \"name\": \"read_file\",\n    \"description\": \"Read a file\",\n    \"parameters\": {\n      \"type\": \"object\"\n    }\n  }\n]"
	dupBlock := strings.Repeat("这是一段跨轮重复出现的超长系统指令或文件内容。", 15) // > 600 bytes
	input := "[System instruction]: You have access to tools.\n\n" + rawToolJSON + "\n\n" +
		dupBlock + "\n\n" +
		"\x1b[32mGreen ANSI Log\x1b[0m\n\n\n\n\n" +
		"========================================\n\n" +
		dupBlock + "\n\n" +
		"请回答最后一个问题。"

	out := compressPromptLossless(input)
	if len(out) >= len(input) {
		t.Fatalf("expected compressed output to be smaller: got %d >= %d", len(out), len(input))
	}
	if strings.Contains(out, "\x1b[") {
		t.Error("ANSI escape code was not stripped")
	}
	if !strings.Contains(out, "Green ANSI Log") {
		t.Error("log text content was lost")
	}
	if strings.Contains(out, "\n  {\n    \"name\": \"read_file\"") {
		t.Error("embedded tool JSON was not compacted")
	}
	if !strings.Contains(out, `[{"name":"read_file"`) {
		t.Error("compacted tool JSON missing")
	}
	if strings.Count(out, dupBlock) != 1 {
		t.Errorf("expected duplicate large block to appear once (latest), got %d", strings.Count(out, dupBlock))
	}
	if !strings.Contains(out, "请回答最后一个问题。") {
		t.Error("latest question was lost")
	}
}

func TestSplitPromptIntoChunksUTF8Safe(t *testing.T) {
	// 构造含大量中文字符的文本，测试切分点绝不劈开 UTF-8 多字节字符
	para := strings.Repeat("临床实证医学与深度学习模型架构分析。", 20) + "\n\n"
	longText := strings.Repeat(para, 30) // ~33,000 bytes
	chunks := splitPromptIntoChunks(longText, 8000)
	if len(chunks) < 4 {
		t.Fatalf("expected at least 4 chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > 8000 {
			t.Errorf("chunk %d exceeds maxChunkBytes: %d > 8000", i, len(c))
		}
		if !utf8.ValidString(c) {
			t.Errorf("chunk %d contains invalid UTF-8 (split mid-rune)", i)
		}
	}
}

func TestCollectFilesAndAudioUploads(t *testing.T) {
	pdfB64 := base64.StdEncoding.EncodeToString([]byte("%PDF-1.7 sample pdf content"))
	audioB64 := base64.StdEncoding.EncodeToString([]byte("ID3 fake mp3 bytes"))
	msgs := []map[string]interface{}{
		{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{
					"type": "file",
					"file": map[string]interface{}{
						"filename":  "report.pdf",
						"file_data": pdfB64,
					},
				},
				map[string]interface{}{
					"type": "input_audio",
					"input_audio": map[string]interface{}{
						"data":   audioB64,
						"format": "mp3",
					},
				},
			},
		},
	}
	uploads, err := collectImages(msgs, "")
	if err != nil {
		t.Fatalf("collectImages failed: %v", err)
	}
	if len(uploads) != 2 {
		t.Fatalf("expected 2 uploads, got %d", len(uploads))
	}
	if uploads[0].Kind != 3 || uploads[0].Mime != "application/pdf" || uploads[0].Name != "report.pdf" {
		t.Errorf("unexpected PDF upload struct: %+v", uploads[0])
	}
	if uploads[1].Kind != 3 || uploads[1].Mime != "audio/mpeg" || uploads[1].Name != "audio2.mp3" {
		t.Errorf("unexpected Audio upload struct: %+v", uploads[1])
	}
}
