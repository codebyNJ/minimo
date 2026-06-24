package pricing

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/codebyNJ/minimo/internal/provider"
)

// perMTok converts a LiteLLM per-token cost to a per-million-token rate.
const perMTok = 1_000_000

type Entry struct {
	InputPerMTok         float64
	OutputPerMTok        float64
	CacheReadPerMTok     float64
	CacheCreationPerMTok float64
	// ContextWindow is the model's input context size in tokens
	// (LiteLLM's max_input_tokens), or 0 when the catalog doesn't list one.
	ContextWindow int
}

type Catalog struct {
	entries    map[string]Entry // raw model key
	normalized map[string]Entry // normalizeModel(key) -> entry, first wins
}

type litellmEntry struct {
	InputCostPerToken         *float64 `json:"input_cost_per_token"`
	OutputCostPerToken        float64  `json:"output_cost_per_token"`
	CacheReadInputTokenCost   float64  `json:"cache_read_input_token_cost"`
	CacheCreationInputTokCost float64  `json:"cache_creation_input_token_cost"`
	MaxInputTokens            flexInt  `json:"max_input_tokens"`
}

// flexInt is a JSON integer that tolerates the value being encoded as a
// string or a float — LiteLLM's catalog does both (and its "sample_spec"
// schema entry even puts a sentence there). Anything unparseable decodes to 0
// without failing the surrounding object, so one malformed field never aborts
// the entire catalog parse.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		*f = flexInt(int(v))
	}
	return nil
}

func parseLiteLLM(data []byte) (Catalog, error) {
	var raw map[string]litellmEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return Catalog{}, err
	}
	cat := Catalog{
		entries:    make(map[string]Entry, len(raw)),
		normalized: make(map[string]Entry, len(raw)),
	}
	for name, le := range raw {
		// Entries without an input cost (e.g. "sample_spec") are not models.
		if le.InputCostPerToken == nil {
			continue
		}
		e := Entry{
			InputPerMTok:         *le.InputCostPerToken * perMTok,
			OutputPerMTok:        le.OutputCostPerToken * perMTok,
			CacheReadPerMTok:     le.CacheReadInputTokenCost * perMTok,
			CacheCreationPerMTok: le.CacheCreationInputTokCost * perMTok,
			ContextWindow:        int(le.MaxInputTokens),
		}
		cat.entries[name] = e
		n := normalizeModel(name)
		if _, exists := cat.normalized[n]; !exists {
			cat.normalized[n] = e
		}
	}
	return cat, nil
}

var dateSuffix = regexp.MustCompile(`-\d{8}$`)

// normalizeModel lowercases and strips a trailing -YYYYMMDD date suffix, so a
// dated model id matches its undated catalog key.
func normalizeModel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return dateSuffix.ReplaceAllString(s, "")
}

// LoadFromBytes parses an in-memory LiteLLM catalog. Used for tests and any
// caller that already holds the JSON.
func LoadFromBytes(data []byte) (Catalog, error) {
	return parseLiteLLM(data)
}

func (c Catalog) Lookup(model string) (Entry, bool) {
	if e, ok := c.entries[model]; ok {
		return e, true
	}
	if e, ok := c.normalized[normalizeModel(model)]; ok {
		return e, true
	}
	return Entry{}, false
}

// ContextWindow returns the model's context-window size in tokens. ok is
// false for an unrecognized model or one the catalog lists without a window,
// so callers never display a percentage against a guessed denominator.
func (c Catalog) ContextWindow(model string) (int, bool) {
	e, ok := c.Lookup(model)
	if !ok || e.ContextWindow <= 0 {
		return 0, false
	}
	return e.ContextWindow, true
}

// Estimate prices the lifetime token categories. Returns ok=false for an
// unrecognized model — never a guessed figure.
func (c Catalog) Estimate(model string, u provider.TokenUsage) (float64, bool) {
	e, ok := c.Lookup(model)
	if !ok {
		return 0, false
	}
	cost := float64(u.Input)/perMTok*e.InputPerMTok +
		float64(u.Output)/perMTok*e.OutputPerMTok +
		float64(u.CacheRead)/perMTok*e.CacheReadPerMTok +
		float64(u.CacheCreation)/perMTok*e.CacheCreationPerMTok
	return cost, true
}
