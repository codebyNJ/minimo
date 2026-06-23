package provider

var registry []Provider

var pathOverrides = map[string]string{}

func Register(p Provider) {
	registry = append(registry, p)
}

func All() []Provider {
	return registry
}

// SetPathOverride records a user-configured root path for the named
// provider. Providers resolve their root lazily (at Detect/ListSessions
// time, not at construction), checking this override before falling back
// to the tool's own env var or hardcoded default — main() calls this
// after config.Load(), but provider self-registration via init() runs
// before that, so the override can't be baked into a struct field at
// construction time.
func SetPathOverride(name, path string) {
	pathOverrides[name] = path
}

func PathOverride(name string) (string, bool) {
	path, ok := pathOverrides[name]
	return path, ok
}
