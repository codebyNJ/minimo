package codex

import (
	"encoding/base64"
	"testing"
)

func makeJWT(payload string) string {
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	return enc(`{"alg":"none"}`) + "." + enc(payload) + ".sig"
}

func TestParsePlanFromJWT(t *testing.T) {
	jwt := makeJWT(`{"https://api.openai.com/auth":{"chatgpt_plan_type":"pro"}}`)
	auth := `{"tokens":{"id_token":"` + jwt + `"}}`
	p := parsePlan([]byte(auth))
	if !p.Known || p.Tier != "Pro" {
		t.Fatalf("got %+v, want {Pro true}", p)
	}
}

func TestParsePlanMissingToken(t *testing.T) {
	if parsePlan([]byte(`{"tokens":{}}`)).Known {
		t.Fatal("missing id_token must yield Known=false")
	}
}

func TestParsePlanMalformedJWT(t *testing.T) {
	auth := `{"tokens":{"id_token":"not-a-jwt"}}`
	if parsePlan([]byte(auth)).Known {
		t.Fatal("malformed JWT must yield Known=false")
	}
}
