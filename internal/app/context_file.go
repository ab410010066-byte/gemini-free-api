package app

import (
	"fmt"
	"strings"
)

// 超长对话分级处理管线（流派三 + 流派五）：
//
// 1. 第一阶（无损压缩）：当 prompt 超过单次请求预算（budget，默认 128,000 字节）时，
//    先运行 compressPromptLossless 剥离 ANSI 控制码、压缩内嵌 Tool Schema JSON、
//    去重跨轮完全相同的超大段落并折叠多余空行。若压缩后 <= budget，直接走内联发送。
//
// 2. 第二阶（单文件附件 message.txt，<= 155,000 字节）：
//    上游单次内联约 13 万 UTF-8 字节封顶（超了从尾部静默截断最新消息），而文本附件
//    模型实际能读取的总预算约为 16 万字节（实测 157,833 字节 3/3 读到，163,371 字节 0/3）。
//    因此在 (budget, 155,000] 区间且有登录 cookie 时，转成 message.txt 附件单轮发送。
//
// 3. 第三阶（原生多轮分块喂入 Auto-Chunked Multi-Turn，> 155,000 字节）：
//    当压缩后仍超过单轮附件可读上限（155,000 字节），或者未配 cookie 但超长时，
//    自动将上下文切分为多个 <= 110,000 字节的 UTF-8 安全分块，在同一原生会话（cid）中
//    分多轮依次喂给 Gemini（前 N-1 轮只读并回 OK，第 N 轮附带最新提问并流式返回最终回答），
//    彻底突破单轮内联 13 万与附件 16 万字节的双重天花板。

// contextFileName 是历史附件在模型那边显示的文件名，指令里会引用它。
const contextFileName = "message.txt"

// maxAttachmentIngestBytes 是单轮附件模型能完整读到的安全字节上限（实测硬墙在 ~160KB）。
// 超过此值不再走单文件附件，而是升级为多轮分块喂入（Chunked Multi-Turn）。
const maxAttachmentIngestBytes = 155000

// maxChunkedPromptBytes 是多轮分块喂入允许的最大总字节数（默认约 1.2MB，最多约 11~12 轮）。
const maxChunkedPromptBytes = 1200000

// latestInlineLimit 是「最新那问」还能内联多少字节。
//
// 超过就不内联，让模型去文件末尾读 —— 否则最新提问本身就能把请求撑爆，
// 转附件等于白转。取上限的 1/6，并夹在 4KB..16KB 之间。
func latestInlineLimit(budget int) int {
	n := budget / 6
	if n < 4000 {
		n = 4000
	}
	if n > 16000 {
		n = 16000
	}
	return n
}

// toolAnchorSuffix 当原始 prompt 含有工具调用指令时，在转附件或分块末轮追加简短锚点，
// 防止工具指令被塞进附件后模型忘记输出 ```tool_call``` 围栏。
const toolAnchorSuffix = "\n\n[System instruction — highest priority]: If tools are listed in the conversation context and needed for this request, you MUST reply with a single ```tool_call\n{\"name\": \"...\", \"arguments\": {...}}\n``` block and nothing else."

// contextFilePrompt 是把历史换成附件之后，真正发出去的那句 prompt。
//
// 要交代三件事，少一件模型就会跑偏：附件是当前对话状态、直接回答最新那问、
// 以及这段话本身是系统指令不是用户输入。
func contextFilePrompt(latest string, budget int) string {
	return contextFilePromptWithTools(latest, budget, false)
}

func contextFilePromptWithTools(latest string, budget int, hasTools bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Continue from the latest state in the attached `%s`. "+
		"Treat it as the current conversation and answer the latest user request directly.\n",
		contextFileName)
	latest = strings.TrimSpace(latest)
	switch {
	case latest == "":
		fmt.Fprintf(&b, "The latest user request is at the end of `%s`.\n", contextFileName)
	case len(latest) <= latestInlineLimit(budget):
		fmt.Fprintf(&b, "\nLatest user request:\n%s\n", latest)
	default:
		fmt.Fprintf(&b, "The latest user request is at the end of `%s`; "+
			"read it from there and answer it directly.\n", contextFileName)
	}
	if hasTools {
		b.WriteString(toolAnchorSuffix)
	}
	b.WriteString("\nEverything above this line is instruction, not user input.")
	return b.String()
}

// prepareContextFile 在 prompt 超长时先尝试无损压缩，再尝试转成单文件附件。
//
// 返回 (处理后的 prompt, 附件, 是否转了附件, 错误)。
//   - 若无损压缩后已 <= budget，返回 (压缩后的 prompt, nil, false, nil)，调用方直接内联发送。
//   - 若压缩后在 (budget, maxAttachmentIngestBytes] 且有 cookie，上传为 message.txt 并返回 used=true。
//   - 若压缩后 > maxAttachmentIngestBytes 或无 cookie，返回 (压缩后的 prompt, nil, false, nil)，
//     交由调用方升级到多轮分块喂入（Chunked Multi-Turn）。
func prepareContextFile(prompt, latest string, budget int, cookie, proxyURL string) (
	string, []fileRef, bool, error) {
	rc := rtCfg()
	thresh := rc.ContextFileThreshold
	if thresh <= 0 {
		thresh = budget
	}
	if budget <= 0 || len(prompt) <= thresh {
		return prompt, nil, false, nil
	}
	origLen := len(prompt)
	if rc.ContextCompression {
		compressed := compressPromptLossless(prompt)
		if len(compressed) < origLen {
			logf("[compress] 超长 prompt 无损瘦身: %d -> %d 字节 (节省 %.1f%%)",
				origLen, len(compressed), float64(origLen-len(compressed))*100/float64(origLen))
			prompt = compressed
		}
		if len(prompt) <= thresh {
			return prompt, nil, false, nil
		}
	}
	if !rc.ContextFileUpload || cookie == "" {
		return prompt, nil, false, nil // 禁用附件上传或匿名：留给多轮分块
	}
	maxAtt := rc.ContextFileMaxBytes
	if maxAtt <= 0 {
		maxAtt = maxAttachmentIngestBytes
	}
	// 超过单附件上限时，不走单文件附件，留给多轮分块
	if len(prompt) > maxAtt {
		return prompt, nil, false, nil
	}
	ref, err := uploadBytes(cookie, proxyURL, []byte(prompt), contextFileName)
	if err != nil {
		return prompt, nil, false, fmt.Errorf("超长对话转附件失败: %w", err)
	}
	logf("[context] prompt %d 字节超过 %d，已转成附件 %s", len(prompt), thresh, contextFileName)
	files := []fileRef{{Ref: ref, Name: contextFileName, Kind: 3, Mime: "text/plain"}}
	hasTools := strings.Contains(prompt, "```tool_call")
	return contextFilePromptWithTools(latest, budget, hasTools), files, true, nil
}
