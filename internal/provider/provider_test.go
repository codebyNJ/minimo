package provider

import "testing"

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
