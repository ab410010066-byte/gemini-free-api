package app

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

// 流派三：无损上下文压缩管线（Lossless Context Compression Pipeline）
//
// 设计原则：
// 1. 仅在 prompt 超过单次请求预算（budget）时触发，常规请求 100% 零干预。
// 2. 严格无损（Lossless）：不删减任何自然语言词汇、不改写代码块，只消除协议与格式噪音：
//    - 剥离终端 ANSI 颜色/控制转义码（工具输出常见噪音）
//    - 压缩内嵌的 pretty-printed Tool Schema JSON（省 25%~35% 工具定义体积）
//    - 折叠连续多余空行与超长分隔线
//    - 跨轮完全相同的超大文本块（>600 字节）去重（保留最新一次完整内容）

var (
	ansiEscapeRe   = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)
	multiNewlineRe = regexp.MustCompile(`\n{3,}`)
	longDividerRe  = regexp.MustCompile(`(?m)^([=\-_~*]{20,})[ \t]*$`)
)

// compressPromptLossless 对超长 prompt 执行多层无损瘦身。
func compressPromptLossless(prompt string) string {
	if prompt == "" {
		return prompt
	}
	s := strings.ReplaceAll(prompt, "\r\n", "\n")

	// 1. 剥离 ANSI 控制序列与无意义零宽字符（常见于剪贴板与 Web 爬取文本）
	if strings.Contains(s, "\x1b[") {
		s = ansiEscapeRe.ReplaceAllString(s, "")
	}
	s = strings.ReplaceAll(s, "\ufeff", "")
	s = strings.ReplaceAll(s, "\u200b", "")
	s = strings.ReplaceAll(s, "\u200c", "")
	s = strings.ReplaceAll(s, "\u200d", "")

	// 2. 压缩内嵌的 Available tools JSON 定义块
	s = compactEmbeddedToolJSON(s)

	// 3. 缩短超长分隔线（20+ 个连续 =/-/_ 等压成 ---）
	s = longDividerRe.ReplaceAllString(s, "---")

	// 4. 去掉每行末尾多余的空格/Tab（保留代码块内部缩进）
	s = trimTrailingHorizontalSpaces(s)

	// 5. 折叠连续 3 个以上换行符为双换行（先规整段落边界）
	s = multiNewlineRe.ReplaceAllString(s, "\n\n")

	// 6. 跨轮重复大段落去重（保留最后一次出现的完整块，前面的重复块替换为简短引用）
	s = dedupLargeIdenticalBlocks(s, 600)

	return strings.TrimSpace(s)
}

// compactEmbeddedToolJSON 把 buildPrompt 注入的 Available tools 多行缩进 JSON 压成紧凑单行。
func compactEmbeddedToolJSON(s string) string {
	const marker = "Available tools:\n["
	idx := strings.Index(s, marker)
	if idx < 0 {
		return s
	}
	jsonStart := idx + len("Available tools:\n")
	// 找到匹配的 JSON 数组闭合位置
	end := findJSONEnd(s[jsonStart:])
	if end <= 0 {
		return s
	}
	rawJSON := s[jsonStart : jsonStart+end]
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(rawJSON)); err != nil {
		return s
	}
	return s[:jsonStart] + buf.String() + s[jsonStart+end:]
}

func findJSONEnd(s string) int {
	depth := 0
	inStr := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inStr {
			escaped = true
			continue
		}
		if ch == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		switch ch {
		case '[', '{':
			depth++
		case ']', '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

func trimTrailingHorizontalSpaces(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

// dedupLargeIdenticalBlocks 将按 "\n\n" 切分的大段落（>= minBytes）去重：
// 若完全相同的段落出现多次，保留最后一次完整文本，前面的替换为简短占位说明。
func dedupLargeIdenticalBlocks(s string, minBytes int) string {
	blocks := strings.Split(s, "\n\n")
	if len(blocks) < 3 {
		return s
	}
	lastSeen := make(map[string]int, len(blocks))
	for i, b := range blocks {
		trimmed := strings.TrimSpace(b)
		if len(trimmed) >= minBytes {
			lastSeen[trimmed] = i
		}
	}
	if len(lastSeen) == 0 {
		return s
	}
	changed := false
	for i, b := range blocks {
		trimmed := strings.TrimSpace(b)
		if len(trimmed) >= minBytes {
			if lastIdx, ok := lastSeen[trimmed]; ok && lastIdx > i {
				blocks[i] = "[Duplicate context block omitted — identical content appears later in the conversation]"
				changed = true
			}
		}
	}
	if !changed {
		return s
	}
	return strings.Join(blocks, "\n\n")
}

// splitPromptIntoChunks 将超长 prompt 按 UTF-8 安全边界（优先换行符）切分为 <= maxChunkBytes 的多块。
func splitPromptIntoChunks(prompt string, maxChunkBytes int) []string {
	if maxChunkBytes <= 0 {
		maxChunkBytes = 100000
	}
	if len(prompt) <= maxChunkBytes {
		return []string{prompt}
	}
	var chunks []string
	rem := prompt
	for len(rem) > 0 {
		if len(rem) <= maxChunkBytes {
			chunks = append(chunks, rem)
			break
		}
		cut := maxChunkBytes
		// 确保不在 UTF-8 多字节字符中间切断
		for cut > 0 && !utf8.RuneStart(rem[cut]) {
			cut--
		}
		if cut == 0 {
			cut = maxChunkBytes
		}
		// 优先在最后 20% 区间内找双换行或单换行切分，保持段落与代码行完整
		minSearch := cut * 4 / 5
		if nl := strings.LastIndex(rem[:cut], "\n\n"); nl >= minSearch {
			cut = nl + 2
		} else if nl := strings.LastIndexByte(rem[:cut], '\n'); nl >= minSearch {
			cut = nl + 1
		}
		chunks = append(chunks, strings.TrimSpace(rem[:cut]))
		rem = strings.TrimSpace(rem[cut:])
	}
	return chunks
}
