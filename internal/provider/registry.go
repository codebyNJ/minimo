package provider

var registry []Provider

func Register(p Provider) {
	registry = append(registry, p)
}

func All() []Provider {
	return registry
}
