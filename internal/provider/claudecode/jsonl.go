package claudecode

import (
	"bufio"
	"bytes"
	"encoding/json"
	"time"
)

type jsonlLine struct {
	Type      string `json:"type"`
	AITitle   string `json:"aiTitle"`
	Timestamp string `json:"timestamp"`
	CWD       string `json:"cwd"`
	Message   struct {
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

func parseLines(data []byte) []jsonlLine {
	var lines []jsonlLine
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		var l jsonlLine
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
