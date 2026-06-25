// Package usage aggregates per-model cost and time-utilization metrics over
// rolling day/week/month windows from the sessions the engine has enumerated.
// It is a pure function of (sessions, now): no persistence, no I/O. The session
// files on disk are the durable history; this just rolls them up.
//
// Attribution model (v1, session-span): a session is placed in a window when
// its LastActive falls inside that window. Cost and token totals are lifetime
// cumulative figures and cannot be split across a window boundary without
// per-turn data, so the whole session total is attributed to its window
// (lumpy at boundaries). Used-time, by contrast, is the session's active span
// clipped to the window and merged across overlapping sessions of the same
// model, so it never exceeds the window length.
package usage

import (
	"sort"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
)

// Window is a rolling lookback period ending at "now".
type Window struct {
	Name     string // short key, e.g. "Today"
	Label    string // human label, e.g. "last 24h"
	Duration time.Duration
}

// DefaultWindows are the rolling periods reported by Build: last 24h / 7d / 30d.
var DefaultWindows = []Window{
	{Name: "Today", Label: "last 24h", Duration: 24 * time.Hour},
	{Name: "Week", Label: "last 7 days", Duration: 7 * 24 * time.Hour},
	{Name: "Month", Label: "last 30 days", Duration: 30 * 24 * time.Hour},
}

// ModelStat is one model's rolled-up activity within a single window.
type ModelStat struct {
	Model     string
	Sessions  int
	TotalCost float64 // sum of known session costs in the window
	CostKnown bool    // false when no session in the group reported a cost
	Estimated bool    // true when any contributing cost was a catalog estimate
	Tokens    int     // sum of lifetime token totals
	UsedTime  time.Duration
	// UsedFraction is UsedTime / Window.Duration, capped at 1.0 — the share of
	// the window during which at least one session of this model was active.
	UsedFraction float64
}

// WindowReport is every active model's stats for one window, sorted by cost.
type WindowReport struct {
	Window Window
	Models []ModelStat
}

// Report is the full set of window reports.
type Report struct {
	Windows []WindowReport
}

// Build rolls the sessions up over the DefaultWindows.
func Build(sessions []provider.SessionContext, now time.Time) Report {
	return BuildWindows(sessions, now, DefaultWindows)
}

// BuildWindows rolls the sessions up over the supplied windows.
func BuildWindows(sessions []provider.SessionContext, now time.Time, windows []Window) Report {
	var rep Report
	for _, w := range windows {
		rep.Windows = append(rep.Windows, buildWindow(sessions, now, w))
	}
	return rep
}

type interval struct{ start, end time.Time }

type modelAcc struct {
	sessions  int
	cost      float64
	costKnown bool
	estimated bool
	tokens    int
	spans     []interval
}

func buildWindow(sessions []provider.SessionContext, now time.Time, w Window) WindowReport {
	winStart := now.Add(-w.Duration)
	byModel := map[string]*modelAcc{}

	for _, s := range sessions {
		last := s.Session.LastActive
		if last.IsZero() || !last.After(winStart) || last.After(now) {
			continue
		}
		model := s.Session.Model
		if model == "" {
			model = "(unknown)"
		}
		a := byModel[model]
		if a == nil {
			a = &modelAcc{}
			byModel[model] = a
		}
		a.sessions++
		a.tokens += s.Tokens.Total
		if s.Cost.Known {
			a.cost += s.Cost.USD
			a.costKnown = true
			if s.Cost.Source == provider.CostSourceEstimated {
				a.estimated = true
			}
		}

		// Clip the active span [StartedAt, LastActive] to the window. A missing
		// or inverted StartedAt degenerates to a zero-length point at LastActive.
		start := s.Session.StartedAt
		if start.IsZero() || start.After(last) {
			start = last
		}
		if start.Before(winStart) {
			start = winStart
		}
		if last.After(start) {
			a.spans = append(a.spans, interval{start, last})
		}
	}

	models := make([]ModelStat, 0, len(byModel))
	for model, a := range byModel {
		used := mergeDuration(a.spans)
		frac := 0.0
		if w.Duration > 0 {
			frac = float64(used) / float64(w.Duration)
			if frac > 1 {
				frac = 1
			}
		}
		models = append(models, ModelStat{
			Model:        model,
			Sessions:     a.sessions,
			TotalCost:    a.cost,
			CostKnown:    a.costKnown,
			Estimated:    a.estimated,
			Tokens:       a.tokens,
			UsedTime:     used,
			UsedFraction: frac,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].TotalCost != models[j].TotalCost {
			return models[i].TotalCost > models[j].TotalCost
		}
		if models[i].UsedTime != models[j].UsedTime {
			return models[i].UsedTime > models[j].UsedTime
		}
		return models[i].Model < models[j].Model
	})
	return WindowReport{Window: w, Models: models}
}

// mergeDuration returns the total length of the union of the spans, so
// overlapping concurrent sessions of one model are not double-counted.
func mergeDuration(spans []interval) time.Duration {
	if len(spans) == 0 {
		return 0
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start.Before(spans[j].start) })
	var total time.Duration
	curStart, curEnd := spans[0].start, spans[0].end
	for _, s := range spans[1:] {
		if s.start.After(curEnd) {
			total += curEnd.Sub(curStart)
			curStart, curEnd = s.start, s.end
			continue
		}
		if s.end.After(curEnd) {
			curEnd = s.end
		}
	}
	total += curEnd.Sub(curStart)
	return total
}
