package usage

import (
	"testing"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
)

func sess(model string, started, last time.Time, usd float64, tokens int, estimated bool) provider.SessionContext {
	src := provider.CostSourceExact
	if estimated {
		src = provider.CostSourceEstimated
	}
	return provider.SessionContext{
		Session: provider.SessionInfo{Model: model, StartedAt: started, LastActive: last},
		Cost:    provider.Cost{USD: usd, Known: true, Source: src},
		Tokens:  provider.TokenUsage{Total: tokens},
	}
}

func windowByName(r Report, name string) WindowReport {
	for _, w := range r.Windows {
		if w.Window.Name == name {
			return w
		}
	}
	return WindowReport{}
}

func modelByName(w WindowReport, model string) (ModelStat, bool) {
	for _, m := range w.Models {
		if m.Model == model {
			return m, true
		}
	}
	return ModelStat{}, false
}

func TestBuildAttributesByWindowAndModel(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	hour := time.Hour
	day := 24 * time.Hour
	sessions := []provider.SessionContext{
		// opus active 2h ago, 1h span -> in all three windows
		sess("opus", now.Add(-3*hour), now.Add(-2*hour), 10, 1000, true),
		// opus active 5d ago, 2h span -> in week + month, not today
		sess("opus", now.Add(-5*day-2*hour), now.Add(-5*day), 20, 2000, true),
		// sonnet active 10d ago, zero span -> month only
		sess("sonnet", now.Add(-10*day), now.Add(-10*day), 5, 500, true),
		// 40d ago -> outside all windows
		sess("opus", now.Add(-40*day), now.Add(-40*day), 99, 9999, true),
	}
	rep := Build(sessions, now)

	today := windowByName(rep, "Today")
	if len(today.Models) != 1 {
		t.Fatalf("Today models = %d, want 1", len(today.Models))
	}
	o, _ := modelByName(today, "opus")
	if o.Sessions != 1 || o.TotalCost != 10 || o.Tokens != 1000 || o.UsedTime != hour {
		t.Fatalf("Today opus = %+v, want 1 session/$10/1000tok/1h", o)
	}

	week := windowByName(rep, "Week")
	ow, _ := modelByName(week, "opus")
	if ow.Sessions != 2 || ow.TotalCost != 30 || ow.Tokens != 3000 || ow.UsedTime != 3*hour {
		t.Fatalf("Week opus = %+v, want 2 sessions/$30/3000tok/3h", ow)
	}
	if _, ok := modelByName(week, "sonnet"); ok {
		t.Fatal("sonnet (10d ago) must not appear in the 7d week window")
	}

	month := windowByName(rep, "Month")
	if _, ok := modelByName(month, "sonnet"); !ok {
		t.Fatal("sonnet (10d ago) must appear in the 30d month window")
	}
	om, _ := modelByName(month, "opus")
	if om.Sessions != 2 {
		t.Fatalf("Month opus sessions = %d, want 2 (40d-ago session excluded)", om.Sessions)
	}
}

func TestUsedFractionIsTimeOverWindow(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	// opus active for exactly 2.4h in the last 24h -> 10% of the day.
	sessions := []provider.SessionContext{
		sess("opus", now.Add(-3*time.Hour), now.Add(-36*time.Minute), 1, 1, true),
	}
	rep := Build(sessions, now)
	o, _ := modelByName(windowByName(rep, "Today"), "opus")
	if o.UsedTime != 144*time.Minute {
		t.Fatalf("used = %v, want 2h24m", o.UsedTime)
	}
	if o.UsedFraction < 0.099 || o.UsedFraction > 0.101 {
		t.Fatalf("fraction = %v, want ~0.10", o.UsedFraction)
	}
}

func TestUsedTimeMergesOverlappingSpans(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	h := time.Hour
	sessions := []provider.SessionContext{
		sess("opus", now.Add(-3*h), now.Add(-1*h), 1, 1, true), // [-3h, -1h]
		sess("opus", now.Add(-2*h), now, 1, 1, true),           // [-2h, 0] overlaps by 1h
	}
	rep := Build(sessions, now)
	o, _ := modelByName(windowByName(rep, "Today"), "opus")
	// Union [-3h, 0] = 3h, not 4h.
	if o.UsedTime != 3*h {
		t.Fatalf("merged used = %v, want 3h (overlap collapsed)", o.UsedTime)
	}
}

func TestUsedTimeClipsToWindowStart(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	// Session started 30h ago, last active 1h ago. Only the in-window portion
	// (last 24h) counts: from 24h-ago to 1h-ago = 23h.
	sessions := []provider.SessionContext{
		sess("opus", now.Add(-30*time.Hour), now.Add(-1*time.Hour), 1, 1, true),
	}
	rep := Build(sessions, now)
	o, _ := modelByName(windowByName(rep, "Today"), "opus")
	if o.UsedTime != 23*time.Hour {
		t.Fatalf("clipped used = %v, want 23h", o.UsedTime)
	}
}

func TestBuildIgnoresZeroLastActive(t *testing.T) {
	now := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	sessions := []provider.SessionContext{
		{Session: provider.SessionInfo{Model: "ghost"}}, // zero LastActive
	}
	rep := Build(sessions, now)
	if len(windowByName(rep, "Today").Models) != 0 {
		t.Fatal("a session with no LastActive timestamp must be ignored")
	}
}
