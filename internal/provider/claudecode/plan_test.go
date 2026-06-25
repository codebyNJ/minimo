package claudecode

import "testing"

func TestParsePlanFromSeatTier(t *testing.T) {
	p := parsePlan([]byte(`{"seatTier":"max","hasAvailableSubscription":true}`))
	if !p.Known || p.Tier != "Max" {
		t.Fatalf("got %+v, want {Max true}", p)
	}
}

func TestParsePlanSubscriberWhenNoSeatTier(t *testing.T) {
	p := parsePlan([]byte(`{"seatTier":null,"hasAvailableSubscription":true}`))
	if !p.Known || p.Tier != "Subscriber" {
		t.Fatalf("got %+v, want {Subscriber true}", p)
	}
}

func TestParsePlanUnknownWhenNoSignal(t *testing.T) {
	// Matches this machine's real ~/.claude.json shape (null seatTier, no sub).
	p := parsePlan([]byte(`{"seatTier":null,"hasAvailableSubscription":false,"organizationRateLimitTier":"default_claude_ai"}`))
	if p.Known {
		t.Fatalf("got %+v, want Known=false", p)
	}
}

func TestParsePlanMalformedJSON(t *testing.T) {
	if parsePlan([]byte(`not json`)).Known {
		t.Fatal("malformed JSON must yield Known=false")
	}
}
