package configprovider

import (
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/codebyNJ/minimo/internal/logging"
)

type spec struct {
	Name    string `yaml:"name"`
	Version int    `yaml:"version"`
	Monitor struct {
		Paths []string `yaml:"paths"`
		Parse struct {
			Format         string `yaml:"format"`
			SessionPattern string `yaml:"session_pattern"`
			TokenPattern   string `yaml:"token_pattern"`
			FilePattern    string `yaml:"file_pattern"`
		} `yaml:"parse"`
	} `yaml:"monitor"`
}

func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return filepath.Join(home, ".ctx", "providers")
}

func LoadAll(dir string) []*Provider {
	matches, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil
	}
	var out []*Provider
	for _, path := range matches {
		p, err := loadOne(path)
		if err != nil {
			// A malformed provider spec drops that whole provider; log it so
			// a user who wrote the YAML can see why it never appeared.
			logging.Errorf("configprovider: skipping %s: %v", path, err)
			continue
		}
		out = append(out, p)
	}
	return out
}

func loadOne(path string) (*Provider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	p := &Provider{spec: s}
	if s.Monitor.Parse.SessionPattern != "" {
		p.sessionRe, err = regexp.Compile(s.Monitor.Parse.SessionPattern)
		if err != nil {
			return nil, err
		}
	}
	if s.Monitor.Parse.TokenPattern != "" {
		p.tokenRe, err = regexp.Compile(s.Monitor.Parse.TokenPattern)
		if err != nil {
			return nil, err
		}
	}
	if s.Monitor.Parse.FilePattern != "" {
		p.fileRe, err = regexp.Compile(s.Monitor.Parse.FilePattern)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}
