package configprovider

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/codebyNJ/minimo/internal/provider"
)

type Provider struct {
	spec      spec
	sessionRe *regexp.Regexp
	tokenRe   *regexp.Regexp
	fileRe    *regexp.Regexp
}

func (p *Provider) Name() string { return p.spec.Name }

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

func (p *Provider) matchedFiles() []string {
	var out []string
	for _, pattern := range p.spec.Monitor.Paths {
		matches, err := filepath.Glob(expandHome(pattern))
		if err != nil {
			continue
		}
		out = append(out, matches...)
	}
	return out
}

func (p *Provider) Detect() bool {
	return len(p.matchedFiles()) > 0
}

func (p *Provider) statusFor(modTime time.Time) provider.SessionStatus {
	if time.Since(modTime) < provider.IdleThreshold {
		return provider.StatusIdle
	}
	return provider.StatusEnded
}

func (p *Provider) sessionInfo(path string) (provider.SessionInfo, []byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return provider.SessionInfo{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return provider.SessionInfo{}, nil, err
	}

	label := ""
	if p.sessionRe != nil {
		if m := p.sessionRe.FindStringSubmatch(string(data)); len(m) > 1 {
			label = m[1]
		}
	}

	return provider.SessionInfo{
		ID:         path,
		Provider:   p.Name(),
		CWD:        filepath.Dir(path),
		Label:      label,
		Status:     p.statusFor(fi.ModTime()),
		StartedAt:  fi.ModTime(),
		LastActive: fi.ModTime(),
	}, data, nil
}

// ListSessions returns only the session IDs (matched file paths). The engine
// reads each session's full detail via ReadContext, so reading file content
// here too would just read every matched file twice per refresh.
func (p *Provider) ListSessions() ([]provider.SessionInfo, error) {
	var out []provider.SessionInfo
	for _, path := range p.matchedFiles() {
		out = append(out, provider.SessionInfo{ID: path, Provider: p.Name()})
	}
	return out, nil
}

func (p *Provider) ReadContext(sessionID string) (*provider.SessionContext, error) {
	info, data, err := p.sessionInfo(sessionID)
	if err != nil {
		return nil, err
	}

	tokens := 0
	if p.tokenRe != nil {
		for _, m := range p.tokenRe.FindAllStringSubmatch(string(data), -1) {
			if len(m) > 1 {
				if n, err := strconv.Atoi(m[1]); err == nil {
					tokens += n
				}
			}
		}
	}

	var files []provider.FileRef
	if p.fileRe != nil {
		seen := make(map[string]bool)
		for _, m := range p.fileRe.FindAllStringSubmatch(string(data), -1) {
			if len(m) > 1 && !seen[m[1]] {
				seen[m[1]] = true
				files = append(files, provider.FileRef{Path: m[1]})
			}
		}
	}

	return &provider.SessionContext{
		Session: info,
		Tokens:  provider.TokenUsage{Total: tokens, Source: provider.TokenSourceEstimated},
		Files:   files,
	}, nil
}
