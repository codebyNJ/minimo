package codex

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/codebyNJ/minimo/internal/format"
	"github.com/codebyNJ/minimo/internal/provider"
)

type codexAuth struct {
	Tokens struct {
		IDToken string `json:"id_token"`
	} `json:"tokens"`
}

func parsePlan(data []byte) provider.PlanInfo {
	var a codexAuth
	if err := json.Unmarshal(data, &a); err != nil || a.Tokens.IDToken == "" {
		return provider.PlanInfo{}
	}
	return planFromJWT(a.Tokens.IDToken)
}

// planFromJWT decodes the JWT payload (middle segment) without signature
// verification — ctx only reads a value the user's own Codex login already
// produced; it authenticates nothing.
func planFromJWT(token string) provider.PlanInfo {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return provider.PlanInfo{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return provider.PlanInfo{}
	}
	var claims struct {
		Auth struct {
			ChatGPTPlanType string `json:"chatgpt_plan_type"`
		} `json:"https://api.openai.com/auth"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Auth.ChatGPTPlanType == "" {
		return provider.PlanInfo{}
	}
	return provider.PlanInfo{Tier: format.PrettifyTier(claims.Auth.ChatGPTPlanType), Known: true}
}

// Plan reads <CODEX_HOME>/auth.json (honoring the path override / env var via
// home()). Best-effort: any failure yields Known=false.
func (p *CodexProvider) Plan() provider.PlanInfo {
	data, err := os.ReadFile(filepath.Join(p.home(), "auth.json"))
	if err != nil {
		return provider.PlanInfo{}
	}
	return parsePlan(data)
}
