package kimicode

import (
	"bufio"
	"bytes"
	"encoding/json"
)

type tokenUsage struct {
	InputOther         int64 `json:"input_other"`
	Output             int64 `json:"output"`
	InputCacheRead     int64 `json:"input_cache_read"`
	InputCacheCreation int64 `json:"input_cache_creation"`
}

func (u tokenUsage) total() int64 {
	return u.InputOther + u.Output + u.InputCacheRead + u.InputCacheCreation
}

type statusFields struct {
	ContextTokens    *int64      `json:"context_tokens"`
	MaxContextTokens *int64      `json:"max_context_tokens"`
	TokenUsage       *tokenUsage `json:"token_usage"`
}

// wireLine covers both possible persisted shapes for a wire.jsonl entry —
// flat fields, or the same fields nested under "payload" — since the
// official docs only document the JSON-RPC EventParams{type, payload}
// envelope and never show the literal persisted line format. Checking
// both sidesteps needing an undocumented discriminator value.
type wireLine struct {
	statusFields
	Payload *statusFields `json:"payload"`
}

func (l wireLine) usable() bool {
	return l.ContextTokens != nil || l.MaxContextTokens != nil || l.TokenUsage != nil ||
		(l.Payload != nil && (l.Payload.ContextTokens != nil || l.Payload.MaxContextTokens != nil || l.Payload.TokenUsage != nil))
}

func (l wireLine) contextTokens() *int64 {
	if l.ContextTokens != nil {
		return l.ContextTokens
	}
	if l.Payload != nil {
		return l.Payload.ContextTokens
	}
	return nil
}

func (l wireLine) maxContextTokens() *int64 {
	if l.MaxContextTokens != nil {
		return l.MaxContextTokens
	}
	if l.Payload != nil {
		return l.Payload.MaxContextTokens
	}
	return nil
}

func (l wireLine) usage() *tokenUsage {
	if l.TokenUsage != nil {
		return l.TokenUsage
	}
	if l.Payload != nil {
		return l.Payload.TokenUsage
	}
	return nil
}

func parseWireLines(data []byte) []wireLine {
	var lines []wireLine
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		var l wireLine
		if err := json.Unmarshal(scanner.Bytes(), &l); err != nil {
			continue
		}
		lines = append(lines, l)
	}
	return lines
}
