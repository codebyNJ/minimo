package format

import "testing"

func TestPrettifyTier(t *testing.T) {
	cases := map[string]string{
		"max":          "Max",
		"pro":          "Pro",
		"team_premium": "Team Premium",
		"":             "",
	}
	for in, want := range cases {
		if got := PrettifyTier(in); got != want {
			t.Fatalf("PrettifyTier(%q) = %q, want %q", in, got, want)
		}
	}
}
