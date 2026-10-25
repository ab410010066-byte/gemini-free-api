package app

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
)

// 读图 / 读视频：把客户端传来的图片或视频上传成附件，再在对话里引用。
//
// 只有登录态可用 —— 匿名能把文件传上去，但一引用就被服务端回 1100。
// 附件类型位：1=图片、2=视频、3=文本文件，跟上传和引用共用一套（见 gemini.go 文件元组）。
// 视频实测（抓包）：上传同一条 resumable、payload 文件元组第 2 位填 2、mime video/mp4，
// 模型能读出视频内容（回「这是一个包含图标动画的短片」）。

// 单张图的大小上限。上游没给明确数字，取一个既能覆盖正常截图、又不至于让一次
// 请求拖太久的值。超了直接报错而不是硬传 —— 传上去被拒的话错误信息更难懂。
const maxImageBytes = 12 * 1024 * 1024

// 视频体积上限。视频 base64 塞进 JSON 请求体，太大会让一次请求拖很久，取一个
// 既能覆盖常见短片、又不至于把请求挂死的值。超了直接报错。
const maxVideoBytes = 50 * 1024 * 1024

// 文档/音频文件体积上限（50MB）。
const maxDocBytes = 50 * 1024 * 1024

// pendingUpload 是还没上传的附件。真正上传要等挑完账号和出口，
// 所以从 handler 到 streamGenerate 之间先这样带着。
type pendingUpload struct {
	Data []byte
	Name string
	Mime string
	Kind int // 1=图片，2=视频，3=文本/PDF/音频/普通文件
}

// collectImages 从 OpenAI 格式的 messages 里把附件（图片、视频、PDF/文档、音频）抠出来。
//
// 认以下写法：
//   - image_url / input_image（含被客户端塞进 image_url 的 data:application/pdf 等文档）
//   - video_url / input_video
//   - file / input_file（OpenAI Chat / Responses 标准文件块）
//   - input_audio（OpenAI 音频输入块）
func collectImages(messages []map[string]interface{}, proxyURL string) ([]pendingUpload, error) {
	var out []pendingUpload
	for _, m := range messages {
		parts, ok := m["content"].([]interface{})
		if !ok {
			continue
		}
		for _, c := range parts {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			var src, customName string
			switch getStr(cm, "type") {
			case "image_url":
				if iu, ok := cm["image_url"].(map[string]interface{}); ok {
					src = getStr(iu, "url")
				} else {
					src = getStr(cm, "image_url")
				}
			case "input_image":
				src = firstNonEmpty(getStr(cm, "image_url"), getStr(cm, "url"), getStr(cm, "data"))
			case "video_url":
				if vu, ok := cm["video_url"].(map[string]interface{}); ok {
					src = getStr(vu, "url")
				} else {
					src = getStr(cm, "video_url")
				}
			case "input_video":
				src = firstNonEmpty(getStr(cm, "video_url"), getStr(cm, "url"), getStr(cm, "data"))
			case "file":
				if fm, ok := cm["file"].(map[string]interface{}); ok {
					customName = getStr(fm, "filename")
					src = firstNonEmpty(getStr(fm, "file_data"), getStr(fm, "url"), getStr(fm, "data"))
				} else {
					src = getStr(cm, "file")
				}
				if src != "" && !strings.HasPrefix(src, "data:") && !strings.HasPrefix(src, "http") {
					mime := guessMimeFromFilename(customName)
					src = "data:" + mime + ";base64," + src
				}
			case "input_file":
				customName = getStr(cm, "filename")
				src = firstNonEmpty(getStr(cm, "file_data"), getStr(cm, "file_url"), getStr(cm, "url"), getStr(cm, "data"))
				if src != "" && !strings.HasPrefix(src, "data:") && !strings.HasPrefix(src, "http") {
					mime := guessMimeFromFilename(customName)
					src = "data:" + mime + ";base64," + src
				}
			case "input_audio":
				if am, ok := cm["input_audio"].(map[string]interface{}); ok {
					dataB64 := getStr(am, "data")
					format := strings.ToLower(strings.TrimSpace(getStr(am, "format")))
					if format == "" {
						format = "mp3"
					}
					mime := "audio/mpeg"
					if format == "wav" {
						mime = "audio/wav"
					} else if format == "ogg" {
						mime = "audio/ogg"
					} else if format != "mp3" {
						mime = "audio/" + format
					}
					if dataB64 != "" {
						src = "data:" + mime + ";base64," + dataB64
					}
				}
			default:
				continue
			}
			if strings.TrimSpace(src) == "" {
				continue
			}
			item, err := materializeMedia(src, customName, proxyURL, len(out)+1)
			if err != nil {
				return nil, err
			}
			out = append(out, item)
		}
	}
	return out, nil
}

func guessMimeFromFilename(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown"):
		return "text/markdown"
	case strings.HasSuffix(lower, ".csv"):
		return "text/csv"
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	case strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm"):
		return "text/html"
	default:
		return "text/plain"
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// materializeImage 保留原函数签名供兼容调用。
func materializeImage(src, proxyURL string, idx int) (pendingUpload, error) {
	return materializeMedia(src, "", proxyURL, idx)
}

// materializeMedia 把一个附件来源变成待上传的字节。
func materializeMedia(src, customName, proxyURL string, idx int) (pendingUpload, error) {
	if strings.HasPrefix(src, "data:") {
		u, err := decodeDataURL(src, idx)
		if err == nil && customName != "" {
			u.Name = sanitizeUploadName(customName)
		}
		return u, err
	}
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		u, err := fetchImage(src, proxyURL, idx)
		if err == nil && customName != "" {
			u.Name = sanitizeUploadName(customName)
		}
		return u, err
	}
	return pendingUpload{}, fmt.Errorf("image %d: unsupported source (want a data: URL or http(s) link)", idx)
}

// decodeDataURL 解析 data:<mime>;base64,<数据>。
func decodeDataURL(src string, idx int) (pendingUpload, error) {
	comma := strings.Index(src, ",")
	if comma < 0 {
		return pendingUpload{}, fmt.Errorf("image %d: malformed data URL", idx)
	}
	meta, payload := src[5:comma], src[comma+1:]
	mime := "image/png"
	if i := strings.Index(meta, ";"); i > 0 {
		mime = meta[:i]
	} else if meta != "" {
		mime = meta
	}
	var data []byte
	var err error
	if strings.Contains(meta, "base64") {
		data, err = base64.StdEncoding.DecodeString(payload)
	} else {
		data = []byte(payload)
	}
	if err != nil {
		return pendingUpload{}, fmt.Errorf("image %d: bad base64: %w", idx, err)
	}
	return newMediaUpload(data, mime, idx)
}

// fetchImage 下载远程图片或文件。走跟正式请求同一个出口。
func fetchImage(src, proxyURL string, idx int) (pendingUpload, error) {
	var body []byte
	var ctype string
	if proxyURL != "" {
		req, err := http.NewRequest("GET", src, nil)
		if err != nil {
			return pendingUpload{}, err
		}
		applyChromeHeaders(req)
		resp, err := getStdlibClient(proxyURL).Do(req)
		if err != nil {
			return pendingUpload{}, fmt.Errorf("image %d: fetch failed: %w", idx, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return pendingUpload{}, fmt.Errorf("image %d: fetch returned HTTP %d", idx, resp.StatusCode)
		}
		ctype = resp.Header.Get("Content-Type")
		body, err = io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
		if err != nil {
			return pendingUpload{}, err
		}
	} else {
		req, err := fhttp.NewRequest("GET", src, nil)
		if err != nil {
			return pendingUpload{}, err
		}
		resp, err := getTLSClient().Do(req)
		if err != nil {
			return pendingUpload{}, fmt.Errorf("image %d: fetch failed: %w", idx, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return pendingUpload{}, fmt.Errorf("image %d: fetch returned HTTP %d", idx, resp.StatusCode)
		}
		ctype = resp.Header.Get("Content-Type")
		body, err = io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
		if err != nil {
			return pendingUpload{}, err
		}
	}
	if i := strings.Index(ctype, ";"); i > 0 {
		ctype = ctype[:i]
	}
	if !strings.HasPrefix(ctype, "image/") && !strings.HasPrefix(ctype, "video/") &&
		!strings.HasPrefix(ctype, "audio/") && !strings.HasPrefix(ctype, "text/") &&
		ctype != "application/pdf" && ctype != "application/json" {
		ctype = "image/png"
	}
	return newMediaUpload(body, ctype, idx)
}

// newMediaUpload 按 mime 决定这是图片、视频还是文档/音频附件：
//   - video/* → 附件类型位 2（上限 maxVideoBytes）；
//   - image/* → 附件类型位 1（上限 maxImageBytes）；
//   - application/pdf, text/*, audio/* 等 → 附件类型位 3（上限 maxDocBytes）。
func newMediaUpload(data []byte, mime string, idx int) (pendingUpload, error) {
	if len(data) == 0 {
		return pendingUpload{}, fmt.Errorf("media %d: empty", idx)
	}
	if strings.HasPrefix(mime, "video/") {
		if len(data) > maxVideoBytes {
			return pendingUpload{}, fmt.Errorf("video %d: %d bytes exceeds the %d-byte limit",
				idx, len(data), maxVideoBytes)
		}
		return pendingUpload{
			Data: data, Mime: mime, Kind: 2,
			Name: fmt.Sprintf("video%d%s", idx, mediaExt(mime)),
		}, nil
	}
	if strings.HasPrefix(mime, "audio/") || strings.HasPrefix(mime, "text/") ||
		mime == "application/pdf" || mime == "application/json" ||
		strings.HasPrefix(mime, "application/vnd.") || strings.HasPrefix(mime, "application/msword") {
		if len(data) > maxDocBytes {
			return pendingUpload{}, fmt.Errorf("file %d: %d bytes exceeds the %d-byte limit",
				idx, len(data), maxDocBytes)
		}
		prefix := "document"
		if strings.HasPrefix(mime, "audio/") {
			prefix = "audio"
		}
		return pendingUpload{
			Data: data, Mime: mime, Kind: 3,
			Name: fmt.Sprintf("%s%d%s", prefix, idx, mediaExt(mime)),
		}, nil
	}
	if len(data) > maxImageBytes {
		return pendingUpload{}, fmt.Errorf("image %d: %d bytes exceeds the %d-byte limit",
			idx, len(data), maxImageBytes)
	}
	return pendingUpload{
		Data: data, Mime: mime, Kind: 1,
		Name: fmt.Sprintf("image%d%s", idx, mediaExt(mime)),
	}, nil
}

// mediaExt 按 mime 给个扩展名。文件名会显示给模型看，扩展名对不上容易让它误判。
func mediaExt(mime string) string {
	switch mime {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/heic":
		return ".heic"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "application/pdf":
		return ".pdf"
	case "text/markdown":
		return ".md"
	case "text/csv":
		return ".csv"
	case "application/json":
		return ".json"
	case "text/html":
		return ".html"
	case "text/plain":
		return ".txt"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "audio/ogg":
		return ".ogg"
	case "audio/mp4", "audio/x-m4a", "audio/m4a":
		return ".m4a"
	default:
		if strings.HasPrefix(mime, "video/") {
			return ".mp4"
		}
		if strings.HasPrefix(mime, "audio/") {
			return ".mp3"
		}
		if strings.HasPrefix(mime, "text/") {
			return ".txt"
		}
		return ".png"
	}
}
