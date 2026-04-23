// Package pagination drives offset/limit iteration for list endpoints.
//
// The Timestripe API exposes offset/limit pagination with a total count.
// Callers pass a Fetcher that performs a single page request; Fetch walks
// the result set up to Options.Limit (or until exhausted when --all is set).
package pagination

import "context"

// DefaultLimit is the number of items returned when --limit is not provided.
// Chosen to be small enough for quick inspection and large enough to be useful.
const DefaultLimit = 30

// pageSize is the per-request window size. The server may cap it lower.
const pageSize = 50

// PageInfo is the list metadata included in JSON/YAML output envelopes.
type PageInfo struct {
	Count    int     `json:"count"              yaml:"count"`
	HasMore  bool    `json:"hasMore"            yaml:"hasMore"`
	Next     *string `json:"next,omitempty"     yaml:"next,omitempty"`
	Previous *string `json:"previous,omitempty" yaml:"previous,omitempty"`
}

// Envelope is the canonical JSON list response shape.
type Envelope[T any] struct {
	PageInfo PageInfo `json:"pageInfo" yaml:"pageInfo"`
	Items    []T      `json:"items"    yaml:"items"`
}

// Page is one page of results as returned by a Fetcher.
type Page[T any] struct {
	Count    int
	Next     *string
	Previous *string
	Results  []T
}

// Fetcher performs a single paginated request.
type Fetcher[T any] func(ctx context.Context, limit, offset int) (*Page[T], error)

// Options tunes the iteration. Zero values mean "use defaults".
type Options struct {
	// Limit is the total number of items to return. Ignored when All is true.
	Limit int
	// Offset is the starting offset into the result set.
	Offset int
	// All iterates until the server reports no more results.
	All bool
}

// Fetch walks pages until the limit is reached or results are exhausted.
func Fetch[T any](ctx context.Context, fn Fetcher[T], opts Options) (*Envelope[T], error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultLimit
	}

	out := &Envelope[T]{}
	var lastPage *Page[T]
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	for {
		window := pageSize
		if !opts.All {
			remaining := limit - len(out.Items)
			if remaining <= 0 {
				break
			}
			if remaining < window {
				window = remaining
			}
		}
		p, err := fn(ctx, window, offset)
		if err != nil {
			return nil, err
		}
		lastPage = p
		out.Items = append(out.Items, p.Results...)
		offset += len(p.Results)
		if p.Next == nil || len(p.Results) == 0 {
			break
		}
	}

	if lastPage != nil {
		out.PageInfo = PageInfo{
			Count:    lastPage.Count,
			Next:     lastPage.Next,
			Previous: lastPage.Previous,
			HasMore:  opts.Offset+len(out.Items) < lastPage.Count,
		}
	}
	return out, nil
}
