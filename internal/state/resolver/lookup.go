package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

// Getter performs API lookups.
type Getter interface {
	Get(ctx context.Context, path string) (interface{}, error)
}

// APIClient is an HTTP client for resolver API lookups. Responses are cached
// by full URL so that repeated calls within a single Resolve invocation do not
// make redundant network requests.
type APIClient struct {
	baseURL string
	token   string
	cache   map[string]interface{} // URL -> parsed JSON response
	http    *http.Client
}

// NewAPIClient returns an APIClient configured with the given base URL and
// bearer token.
func NewAPIClient(baseURL, token string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		token:   token,
		cache:   make(map[string]interface{}),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Get makes a GET request to baseURL+path with an Authorization: Bearer header
// and returns the parsed JSON body as interface{} (map or slice). Responses are
// cached by full URL. On HTTP 429 the call is retried with exponential backoff
// and jitter up to 5 times.
func (c *APIClient) Get(ctx context.Context, path string) (interface{}, error) {
	url := c.baseURL + path

	if cached, ok := c.cache[url]; ok {
		return cached, nil
	}

	const maxRetries = 5
	backoff := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("lookup: build request for %q: %w", url, err)
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("lookup: GET %q: %w", url, err)
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries {
			_ = resp.Body.Close()
			// Exponential backoff with jitter: sleep backoff ± 20%.
			jitter := time.Duration(rand.Int63n(int64(backoff) / 5))
			sleep := backoff + jitter
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(sleep):
			}
			backoff *= 2
			continue
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("lookup: GET %q returned HTTP %d", url, resp.StatusCode)
		}

		var result interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("lookup: decode response from %q: %w", url, err)
		}
		_ = resp.Body.Close()

		c.cache[url] = result
		return result, nil
	}

	return nil, fmt.Errorf("lookup: GET %q: exceeded %d retries on HTTP 429", url, maxRetries)
}
