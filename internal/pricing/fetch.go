package pricing

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"time"
)

//go:embed snapshot.json
var snapshotBytes []byte

const catalogURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"

// Load fetches the live LiteLLM catalog once, falling back to the embedded
// snapshot on any failure (network, non-200, malformed JSON). Never fatal.
func Load(ctx context.Context) Catalog {
	if cat, err := fetchLive(ctx); err == nil && len(cat.entries) > 0 {
		return cat
	}
	cat, _ := parseLiteLLM(snapshotBytes)
	return cat
}

func fetchLive(ctx context.Context) (Catalog, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, catalogURL, nil)
	if err != nil {
		return Catalog{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Catalog{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Catalog{}, fmt.Errorf("pricing: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return Catalog{}, err
	}
	return parseLiteLLM(data)
}
