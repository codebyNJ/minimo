package provider

import "testing"

func TestParseTimestampValidRFC3339(t *testing.T) {
	ts, ok := ParseTimestamp("2026-06-24T13:05:00Z")
	if !ok {
		t.Fatal("valid RFC3339 must parse")
	}
	if ts.Year() != 2026 || ts.Month() != 6 || ts.Day() != 24 {
		t.Fatalf("parsed wrong instant: %v", ts)
	}
}

func TestParseTimestampMalformedReportsNotOK(t *testing.T) {
	for _, in := range []string{"", "not-a-time", "2026/06/24 13:05", "1719234300"} {
		if ts, ok := ParseTimestamp(in); ok {
			t.Fatalf("ParseTimestamp(%q) = (%v, true), want ok=false", in, ts)
		}
	}
}

func TestCostSourceZeroValueIsExact(t *testing.T) {
	// An unset Cost.Source must mean "exact" so that providers reporting
	// native cost without setting Source are never mislabeled "estimated".
	if CostSourceExact != 0 {
		t.Fatalf("CostSourceExact must be the zero value, got %d", CostSourceExact)
	}
	var c Cost
	if c.Source != CostSourceExact {
		t.Fatalf("zero Cost.Source = %d, want CostSourceExact", c.Source)
	}
}

func TestTokenUsageHasCategoryFields(t *testing.T) {
	u := TokenUsage{Total: 10, Input: 4, Output: 3, CacheRead: 2, CacheCreation: 1}
	if u.Input+u.Output+u.CacheRead+u.CacheCreation != 10 {
		t.Fatalf("category fields do not sum to total")
	}
}

func TestPlanInfoZeroValueUnknown(t *testing.T) {
	var p PlanInfo
	if p.Known {
		t.Fatalf("zero PlanInfo must be Known=false")
	}
}
