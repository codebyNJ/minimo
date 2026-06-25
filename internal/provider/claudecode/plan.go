package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/codebyNJ/minimo/internal/format"
	"github.com/codebyNJ/minimo/internal/provider"
)

// claudeConfig holds only the non-secret plan-tier fields of ~/.claude.json.
// Go's encoding/json tolerates the duplicate project keys that break stricter
// parsers, since unknown fields are ignored.
type claudeConfig struct {
	SeatTier                 *string `json:"seatTier"`
	HasAvailableSubscription bool    `json:"hasAvailableSubscription"`
}

func parsePlan(data []byte) provider.PlanInfo {
	var c claudeConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return provider.PlanInfo{}
	}
	if c.SeatTier != nil && *c.SeatTier != "" {
		return provider.PlanInfo{Tier: format.PrettifyTier(*c.SeatTier), Known: true}
	}
	if c.HasAvailableSubscription {
		return provider.PlanInfo{Tier: "Subscriber", Known: true}
	}
	return provider.PlanInfo{}
}

// Plan reads ~/.claude.json (distinct from the secret ~/.claude/.credentials.json,
// which is never read). Best-effort: any failure yields Known=false.
func (p *ClaudeCodeProvider) Plan() provider.PlanInfo {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return provider.PlanInfo{}
	}
	data, err := os.ReadFile(filepath.Join(homeDir, ".claude.json"))
	if err != nil {
		return provider.PlanInfo{}
	}
	return parsePlan(data)
}
