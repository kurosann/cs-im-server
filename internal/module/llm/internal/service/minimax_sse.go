package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

var (
	openReasoningTag  = regexp.MustCompile("(?is)<\\s*(think|thinking|analysis|reasoning)\\b[^>]*>")
	closeReasoningTag = regexp.MustCompile("(?is)</\\s*(think|thinking|analysis|reasoning)\\s*>")
)

const maxSSELineBytes = 2 << 20

type reasoningStreamFilter struct {
	carry       string
	inReasoning bool
}

func (f *reasoningStreamFilter) push(fragment string, final bool) (string, string) {
	f.carry += fragment
	var content, reasoning strings.Builder
	for f.carry != "" {
		if f.inReasoning {
			match := closeReasoningTag.FindStringIndex(f.carry)
			if match != nil {
				reasoning.WriteString(f.carry[:match[0]])
				f.carry = f.carry[match[1]:]
				f.inReasoning = false
				continue
			}
			if final {
				f.carry = ""
				break
			}
			start := partialReasoningTagStart(f.carry, true)
			if start < 0 {
				reasoning.WriteString(f.carry)
				f.carry = ""
				break
			}
			reasoning.WriteString(f.carry[:start])
			f.carry = f.carry[start:]
			break
		}
		match := openReasoningTag.FindStringIndex(f.carry)
		if match != nil {
			content.WriteString(f.carry[:match[0]])
			f.carry = f.carry[match[1]:]
			f.inReasoning = true
			continue
		}
		if closeTag := closeReasoningTag.FindStringIndex(f.carry); closeTag != nil {
			content.WriteString(f.carry[:closeTag[0]])
			f.carry = f.carry[closeTag[1]:]
			continue
		}
		if final {
			start := partialReasoningTagStart(f.carry, false)
			if start >= 0 {
				content.WriteString(f.carry[:start])
			} else {
				content.WriteString(f.carry)
			}
			f.carry = ""
			break
		}
		start := partialReasoningTagStart(f.carry, false)
		if start < 0 {
			content.WriteString(f.carry)
			f.carry = ""
			break
		}
		content.WriteString(f.carry[:start])
		f.carry = f.carry[start:]
		break
	}
	return content.String(), reasoning.String()
}

func partialReasoningTagStart(value string, inReasoning bool) int {
	start := strings.LastIndex(value, "<")
	if start < 0 {
		return -1
	}
	suffix := strings.TrimLeft(value[start+1:], " \t\r\n")
	closing := strings.HasPrefix(suffix, "/")
	if closing {
		if !inReasoning {
			return -1
		}
		suffix = strings.TrimLeft(suffix[1:], " \t\r\n")
	}
	lower := strings.ToLower(suffix)
	for _, name := range []string{"think", "thinking", "analysis", "reasoning"} {
		if len(lower) <= len(name) && strings.EqualFold(name[:len(lower)], lower) {
			return start
		}
		if len(lower) > len(name) && strings.EqualFold(lower[:len(name)], name) {
			next := lower[len(name)]
			if next == '>' || next == ' ' || next == '\t' || next == '\r' || next == '\n' {
				return start
			}
		}
	}
	return -1
}

type miniMaxSSEBody struct {
	reader  *bufio.Reader
	closer  io.Closer
	filters map[int]*reasoningStreamFilter
	pending []byte
	eof     bool
}

func newMiniMaxSSEBody(body io.ReadCloser) io.ReadCloser {
	return &miniMaxSSEBody{reader: bufio.NewReader(body), closer: body}
}

func (b *miniMaxSSEBody) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	for len(b.pending) == 0 && !b.eof {
		line, err := b.reader.ReadString('\n')
		if len(line) > maxSSELineBytes {
			return 0, ErrTooLarge
		}
		if len(line) > 0 {
			b.pending = append(b.pending, b.transformLine(line)...)
		}
		if err != nil {
			if err != io.EOF {
				return 0, err
			}
			b.eof = true
			b.pending = append(b.pending, b.flushFilter()...)
		}
	}
	if len(b.pending) == 0 && b.eof {
		return 0, io.EOF
	}
	n := copy(buffer, b.pending)
	b.pending = b.pending[n:]
	return n, nil
}

func (b *miniMaxSSEBody) transformLine(line string) []byte {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "data:") {
		return []byte(line)
	}
	data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
	if data == "[DONE]" {
		b.pending = append(b.pending, b.flushFilter()...)
		return []byte(line)
	}
	if data == "" {
		return []byte(line)
	}
	var payload map[string]any
	if json.Unmarshal([]byte(data), &payload) != nil {
		return []byte(line)
	}
	choices, _ := payload["choices"].([]any)
	modified := false
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]any)
		delta, _ := choice["delta"].(map[string]any)
		if delta == nil {
			continue
		}
		if _, exists := delta["reasoning_content"]; exists {
			delete(delta, "reasoning_content")
			modified = true
		}
		fragment, ok := delta["content"].(string)
		if !ok || fragment == "" {
			text, exists := delta["text"].(string)
			fragment, ok = text, exists
		}
		if !ok || fragment == "" {
			continue
		}
		choiceIndex, _ := choice["index"].(float64)
		index := int(choiceIndex)
		if b.filters == nil {
			b.filters = make(map[int]*reasoningStreamFilter)
		}
		filter := b.filters[index]
		if filter == nil {
			filter = &reasoningStreamFilter{}
			b.filters[index] = filter
		}
		content, reasoning := filter.push(fragment, false)
		if content == "" {
			delete(delta, "content")
			delete(delta, "text")
		} else if _, hasContent := delta["content"]; hasContent {
			delta["content"] = content
		} else {
			delta["text"] = content
		}
		_ = reasoning // Thinking text must not be exposed through the chat response.
		modified = true
	}
	if !modified {
		return []byte(line)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return []byte(line)
	}
	ending := ""
	if strings.HasSuffix(line, "\r\n") {
		ending = "\r\n"
	} else if strings.HasSuffix(line, "\n") {
		ending = "\n"
	}
	return []byte("data: " + string(encoded) + ending)
}

type miniMaxAnthropicSSEBody struct {
	reader         *bufio.Reader
	closer         io.Closer
	filter         reasoningStreamFilter
	eventLines     []string
	lastBlockIndex any
	hiddenBlocks   map[string]struct{}
	pending        []byte
	eof            bool
}

func newMiniMaxAnthropicSSEBody(body io.ReadCloser) io.ReadCloser {
	return &miniMaxAnthropicSSEBody{reader: bufio.NewReader(body), closer: body}
}

func (b *miniMaxAnthropicSSEBody) Read(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	for len(b.pending) == 0 && !b.eof {
		line, err := b.reader.ReadString('\n')
		if len(line) > maxSSELineBytes {
			return 0, ErrTooLarge
		}
		if len(line) > 0 {
			b.eventLines = append(b.eventLines, line)
			if strings.TrimSpace(line) == "" {
				b.pending = append(b.pending, b.transformEvent(b.eventLines)...)
				b.eventLines = nil
			}
		}
		if err != nil {
			if err != io.EOF {
				return 0, err
			}
			b.eof = true
			if len(b.eventLines) > 0 {
				b.pending = append(b.pending, b.transformEvent(b.eventLines)...)
				b.eventLines = nil
			}
			b.pending = append(b.pending, b.flush()...)
		}
	}
	if len(b.pending) == 0 && b.eof {
		return 0, io.EOF
	}
	n := copy(buffer, b.pending)
	b.pending = b.pending[n:]
	return n, nil
}

func (b *miniMaxAnthropicSSEBody) transformEvent(lines []string) []byte {
	var eventName string
	var dataLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(trimmed, "event:"))
		case strings.HasPrefix(trimmed, "data:"):
			dataLines = append(dataLines, strings.TrimPrefix(trimmed, "data:"))
		}
	}
	if len(dataLines) != 1 {
		return []byte(strings.Join(lines, ""))
	}
	var payload map[string]any
	if json.Unmarshal([]byte(strings.TrimSpace(dataLines[0])), &payload) != nil {
		return []byte(strings.Join(lines, ""))
	}
	indexKey := fmt.Sprint(payload["index"])
	if payload["type"] == "content_block_start" {
		block, _ := payload["content_block"].(map[string]any)
		if block != nil && (block["type"] == "thinking" || block["type"] == "redacted_thinking") {
			if b.hiddenBlocks == nil {
				b.hiddenBlocks = make(map[string]struct{})
			}
			b.hiddenBlocks[indexKey] = struct{}{}
			return nil
		}
	}
	if _, hidden := b.hiddenBlocks[indexKey]; hidden {
		if payload["type"] == "content_block_stop" {
			delete(b.hiddenBlocks, indexKey)
		}
		return nil
	}
	if payload["type"] != "content_block_delta" {
		return []byte(strings.Join(lines, ""))
	}
	delta, _ := payload["delta"].(map[string]any)
	if delta == nil {
		return []byte(strings.Join(lines, ""))
	}
	if delta["type"] == "thinking_delta" || delta["type"] == "signature_delta" {
		return nil
	}
	if delta["type"] != "text_delta" {
		return []byte(strings.Join(lines, ""))
	}
	fragment, _ := delta["text"].(string)
	if fragment == "" {
		return []byte(strings.Join(lines, ""))
	}
	content, _ := b.filter.push(fragment, false)
	if content == "" {
		return nil
	}
	delta["text"] = content
	b.lastBlockIndex = payload["index"]
	encoded, err := json.Marshal(payload)
	if err != nil {
		return []byte(strings.Join(lines, ""))
	}
	var output strings.Builder
	if eventName != "" {
		output.WriteString("event: ")
		output.WriteString(eventName)
		output.WriteByte('\n')
	}
	output.WriteString("data: ")
	output.Write(encoded)
	output.WriteString("\n\n")
	return []byte(output.String())
}

func (b *miniMaxAnthropicSSEBody) flush() []byte {
	content, _ := b.filter.push("", true)
	var output []byte
	if content != "" {
		event := map[string]any{"type": "content_block_delta", "index": b.lastBlockIndex, "delta": map[string]any{"type": "text_delta", "text": content}}
		encoded, _ := json.Marshal(event)
		if b.lastBlockIndex == nil {
			return nil
		}
		output = append(output, []byte("event: content_block_delta\ndata: "+string(encoded)+"\n\n")...)
	}
	return output
}

func (b *miniMaxAnthropicSSEBody) Close() error { return b.closer.Close() }

var _ io.ReadCloser = (*miniMaxAnthropicSSEBody)(nil)

func (b *miniMaxSSEBody) flushFilter() []byte {
	var output []byte
	indices := make([]int, 0, len(b.filters))
	for index := range b.filters {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	for _, index := range indices {
		content, _ := b.filters[index].push("", true)
		delta := make(map[string]string)
		if content != "" {
			delta["content"] = content
		}
		if len(delta) == 0 {
			continue
		}
		event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": index, "delta": delta}}})
		output = append(output, []byte("data: "+string(event)+"\n\n")...)
	}
	return output
}

func (b *miniMaxSSEBody) Close() error { return b.closer.Close() }
