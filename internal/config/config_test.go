package config

import "testing"

func TestDefaultYAMLRoundTrips(t *testing.T) {
	data, err := DefaultYAML()
	if err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := unmarshal(data, &c); err != nil {
		t.Fatalf("default YAML failed to parse: %v", err)
	}
	if c.PollIntervalSec != Default().PollIntervalSec || c.DebounceMS != Default().DebounceMS {
		t.Fatalf("round-trip mismatch: %+v vs %+v", c, Default())
	}
}
