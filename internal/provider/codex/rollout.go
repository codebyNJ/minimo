package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"time"
)

type rolloutLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type sessionMetaPayload struct {
	CWD       string `json:"cwd"`
	Timestamp string `json:"timestamp"`
}

type turnContextPayload struct {
	CWD   string `json:"cwd"`
	Model string `json:"model"`
}

type tokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

type tokenUsageInfo struct {
	TotalTokenUsage    tokenUsage `json:"total_token_usage"`
	LastTokenUsage     tokenUsage `json:"last_token_usage"`
	ModelContextWindow *int64     `json:"model_context_window"`
}

type eventMsgPayload struct {
	Type string          `json:"type"`
	Info *tokenUsageInfo `json:"info"`
}

func parseRolloutLines(data []byte) []rolloutLine {
	var lines []rolloutLine
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		var l rolloutLine
		if err := json.Unmarshal(scanner.Bytes(), &l); err != nil {
			continue
		}
		lines = append(lines, l)
	}
	return lines
}

func parseTimestamp(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
