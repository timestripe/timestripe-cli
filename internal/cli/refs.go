package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

// searchWindow is how many search hits we consider before falling back to a
// full walk. A window that comes back full may have been truncated.
const searchWindow = 50

// resource describes how to look one kind of thing up by ID or by name.
//
// page is the single adapter each resource supplies: a paginated list call that
// optionally takes a search term. searchable is false for /folders/, which has
// no search query parameter.
type resource[T any] struct {
	kind       string
	searchable bool
	getByID    func(ctx context.Context, id string) (*T, error)
	page       func(ctx context.Context, limit, offset int, search *string) (*pagination.Page[T], error)
	idOf       func(T) string
	nameOf     func(T) string
	// labelOf renders a candidate for the disambiguation prompt. Falls back to
	// nameOf when nil.
	labelOf func(T) string
}

func (r resource[T]) label(v T) string {
	if r.labelOf != nil {
		return r.labelOf(v)
	}
	return r.nameOf(v)
}

// isNotFound reports whether err is a 404. Only a 404 means "not an ID";
// a 401, 403, 5xx, or transport failure is a real error that must surface
// rather than silently degrading into a name scan.
func isNotFound(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == 404
}

// resolve turns a user-supplied value into a canonical resource ID.
//
//  1. Probe GET by ID, so an ID always beats a resource that happens to share
//     its name. Skipped when the value contains whitespace: no ID does, and it
//     saves a guaranteed 404.
//  2. Match by exact name among search hits (or, for folders, the full list).
//  3. On an ambiguous or missing match, prompt when stdin is a terminal;
//     otherwise return an error naming the candidates.
func resolve[T any](ctx context.Context, cmd *cobra.Command, r resource[T], value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("empty %s reference", r.kind)
	}

	if !strings.ContainsAny(value, " \t") {
		v, err := r.getByID(ctx, value)
		switch {
		case err == nil && v != nil:
			return r.idOf(*v), nil
		case err != nil && !isNotFound(err):
			return "", err
		}
	}

	candidates, err := r.candidates(ctx, value)
	if err != nil {
		return "", err
	}

	matches := make([]T, 0, 2)
	for _, it := range candidates {
		if r.nameOf(it) == value {
			matches = append(matches, it)
		}
	}

	// A full search window may have been truncated, so a miss is not conclusive.
	if len(matches) == 0 && r.searchable && len(candidates) >= searchWindow {
		all, err := r.listAll(ctx)
		if err != nil {
			return "", err
		}
		candidates = all
		for _, it := range all {
			if r.nameOf(it) == value {
				matches = append(matches, it)
			}
		}
	}

	switch len(matches) {
	case 1:
		return r.idOf(matches[0]), nil
	case 0:
		return r.noMatch(cmd, value, candidates)
	default:
		return r.ambiguous(cmd, value, matches)
	}
}

// candidates returns the plausible matches for value: search hits where the
// endpoint supports search, the full list otherwise.
func (r resource[T]) candidates(ctx context.Context, value string) ([]T, error) {
	if !r.searchable {
		return r.listAll(ctx)
	}
	p, err := r.page(ctx, searchWindow, 0, &value)
	if err != nil {
		return nil, err
	}
	return p.Results, nil
}

func (r resource[T]) listAll(ctx context.Context) ([]T, error) {
	env, err := pagination.Fetch(ctx, func(ctx context.Context, limit, offset int) (*pagination.Page[T], error) {
		return r.page(ctx, limit, offset, nil)
	}, pagination.Options{All: true})
	if err != nil {
		return nil, err
	}
	return env.Items, nil
}

// ambiguous handles several exact name matches.
func (r resource[T]) ambiguous(cmd *cobra.Command, value string, matches []T) (string, error) {
	if interactive(cmd) {
		return pick(cmd, fmt.Sprintf("%q matches %d %ss:", value, len(matches), r.kind), r.choices(matches))
	}
	return "", fmt.Errorf("%q matches %d %ss; disambiguate with an ID:\n%s",
		value, len(matches), r.kind, r.bullets(matches))
}

// noMatch handles zero exact name matches, offering near misses when the
// search turned any up.
func (r resource[T]) noMatch(cmd *cobra.Command, value string, candidates []T) (string, error) {
	if len(candidates) == 0 {
		return "", fmt.Errorf("no %s matches %q (not a valid ID and no name match)", r.kind, value)
	}
	if len(candidates) > searchWindow {
		candidates = candidates[:searchWindow]
	}
	if interactive(cmd) {
		return pick(cmd, fmt.Sprintf("no %s is named exactly %q. Close matches:", r.kind, value), r.choices(candidates))
	}
	if len(candidates) == 1 {
		return "", fmt.Errorf("no %s matches %q; did you mean %q (%s)?",
			r.kind, value, r.nameOf(candidates[0]), r.idOf(candidates[0]))
	}
	return "", fmt.Errorf("no %s matches %q exactly; close matches:\n%s", r.kind, value, r.bullets(candidates))
}

func (r resource[T]) choices(items []T) []choice {
	out := make([]choice, 0, len(items))
	for _, it := range items {
		out = append(out, choice{ID: r.idOf(it), Label: r.label(it)})
	}
	return out
}

func (r resource[T]) bullets(items []T) string {
	var b strings.Builder
	for i, it := range items {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "  %s  %s", r.label(it), r.idOf(it))
	}
	return b.String()
}

// Concrete resources. Each supplies one get-by-ID and one paginated list call.

func spaceResource(c *api.ClientWithResponses) resource[api.Space] {
	return resource[api.Space]{
		kind:       "space",
		searchable: true,
		getByID: func(ctx context.Context, id string) (*api.Space, error) {
			resp, err := c.SpacesRetrieveWithResponse(ctx, id)
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return resp.JSON200, nil
		},
		page: func(ctx context.Context, limit, offset int, search *string) (*pagination.Page[api.Space], error) {
			resp, err := c.SpacesListWithResponse(ctx, &api.SpacesListParams{Limit: &limit, Offset: &offset, Search: search})
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return &pagination.Page[api.Space]{Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous, Results: resp.JSON200.Results}, nil
		},
		idOf:   func(s api.Space) string { return ptrStr(s.Id) },
		nameOf: func(s api.Space) string { return ptrStr(s.Name) },
	}
}

func boardResource(c *api.ClientWithResponses) resource[api.Board] {
	return resource[api.Board]{
		kind:       "board",
		searchable: true,
		getByID: func(ctx context.Context, id string) (*api.Board, error) {
			resp, err := c.BoardsRetrieveWithResponse(ctx, id)
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return resp.JSON200, nil
		},
		page: func(ctx context.Context, limit, offset int, search *string) (*pagination.Page[api.Board], error) {
			resp, err := c.BoardsListWithResponse(ctx, &api.BoardsListParams{Limit: &limit, Offset: &offset, Search: search})
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return &pagination.Page[api.Board]{Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous, Results: resp.JSON200.Results}, nil
		},
		idOf:   func(b api.Board) string { return ptrStr(b.Id) },
		nameOf: func(b api.Board) string { return ptrStr(b.Name) },
		labelOf: func(b api.Board) string {
			return withContext(ptrStr(b.Name), "space "+b.SpaceId)
		},
	}
}

func bucketResource(c *api.ClientWithResponses) resource[api.Bucket] {
	return resource[api.Bucket]{
		kind:       "bucket",
		searchable: true,
		getByID: func(ctx context.Context, id string) (*api.Bucket, error) {
			resp, err := c.BucketsRetrieveWithResponse(ctx, id)
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return resp.JSON200, nil
		},
		page: func(ctx context.Context, limit, offset int, search *string) (*pagination.Page[api.Bucket], error) {
			resp, err := c.BucketsListWithResponse(ctx, &api.BucketsListParams{Limit: &limit, Offset: &offset, Search: search})
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return &pagination.Page[api.Bucket]{Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous, Results: resp.JSON200.Results}, nil
		},
		idOf:   func(b api.Bucket) string { return ptrStr(b.Id) },
		nameOf: func(b api.Bucket) string { return ptrStr(b.Name) },
		labelOf: func(b api.Bucket) string {
			return withContext(ptrStr(b.Name), "board "+b.BoardId)
		},
	}
}

func goalResource(c *api.ClientWithResponses) resource[api.Goal] {
	return resource[api.Goal]{
		kind:       "goal",
		searchable: true,
		getByID: func(ctx context.Context, id string) (*api.Goal, error) {
			resp, err := c.GoalsRetrieveWithResponse(ctx, id)
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return resp.JSON200, nil
		},
		page: func(ctx context.Context, limit, offset int, search *string) (*pagination.Page[api.Goal], error) {
			resp, err := c.GoalsListWithResponse(ctx, &api.GoalsListParams{Limit: &limit, Offset: &offset, Search: search})
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return &pagination.Page[api.Goal]{Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous, Results: resp.JSON200.Results}, nil
		},
		idOf:   func(g api.Goal) string { return ptrStr(g.Id) },
		nameOf: func(g api.Goal) string { return ptrStr(g.Name) },
		labelOf: func(g api.Goal) string {
			var bits []string
			if g.Horizon != nil && string(*g.Horizon) != "" {
				bits = append(bits, string(*g.Horizon))
			}
			if g.Date != nil {
				bits = append(bits, g.Date.Time.Format("2006-01-02"))
			}
			if g.Checked != nil && *g.Checked {
				bits = append(bits, "done")
			}
			return withContext(ptrStr(g.Name), strings.Join(bits, ", "))
		},
	}
}

// folderResource has searchable=false: /folders/ has no search query param, so
// name lookups must walk the full list.
func folderResource(c *api.ClientWithResponses) resource[api.Folder] {
	return resource[api.Folder]{
		kind:       "folder",
		searchable: false,
		getByID: func(ctx context.Context, id string) (*api.Folder, error) {
			resp, err := c.FoldersRetrieveWithResponse(ctx, id)
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return resp.JSON200, nil
		},
		page: func(ctx context.Context, limit, offset int, _ *string) (*pagination.Page[api.Folder], error) {
			resp, err := c.FoldersListWithResponse(ctx, &api.FoldersListParams{Limit: &limit, Offset: &offset})
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return &pagination.Page[api.Folder]{Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous, Results: resp.JSON200.Results}, nil
		},
		idOf:   func(f api.Folder) string { return ptrStr(f.Id) },
		nameOf: func(f api.Folder) string { return ptrStr(f.Name) },
	}
}

// userResource matches on email first, then "First Last".
func userResource(c *api.ClientWithResponses) resource[api.User] {
	name := func(u api.User) string {
		if u.Email != nil && string(*u.Email) != "" {
			return string(*u.Email)
		}
		first := ptrStr(u.FirstName)
		last := ptrStr(u.LastName)
		switch {
		case first != "" && last != "":
			return first + " " + last
		case first != "":
			return first
		default:
			return last
		}
	}
	return resource[api.User]{
		kind:       "user",
		searchable: true,
		getByID: func(ctx context.Context, id string) (*api.User, error) {
			resp, err := c.UsersRetrieveWithResponse(ctx, id)
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return resp.JSON200, nil
		},
		page: func(ctx context.Context, limit, offset int, search *string) (*pagination.Page[api.User], error) {
			resp, err := c.UsersListWithResponse(ctx, &api.UsersListParams{Limit: &limit, Offset: &offset, Search: search})
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return &pagination.Page[api.User]{Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous, Results: resp.JSON200.Results}, nil
		},
		idOf:   func(u api.User) string { return ptrStr(u.Id) },
		nameOf: name,
		labelOf: func(u api.User) string {
			full := strings.TrimSpace(ptrStr(u.FirstName) + " " + ptrStr(u.LastName))
			return withContext(name(u), full)
		},
	}
}

// withContext appends a parenthesised qualifier when there is one.
func withContext(label, extra string) string {
	if extra == "" || extra == label {
		return label
	}
	return label + " (" + extra + ")"
}

// Thin wrappers so command code reads the same as before.

func resolveSpaceRef(ctx context.Context, cmd *cobra.Command, c *api.ClientWithResponses, v string) (string, error) {
	return resolve(ctx, cmd, spaceResource(c), v)
}

func resolveBoardRef(ctx context.Context, cmd *cobra.Command, c *api.ClientWithResponses, v string) (string, error) {
	return resolve(ctx, cmd, boardResource(c), v)
}

func resolveBucketRef(ctx context.Context, cmd *cobra.Command, c *api.ClientWithResponses, v string) (string, error) {
	return resolve(ctx, cmd, bucketResource(c), v)
}

func resolveGoalRef(ctx context.Context, cmd *cobra.Command, c *api.ClientWithResponses, v string) (string, error) {
	return resolve(ctx, cmd, goalResource(c), v)
}

func resolveFolderRef(ctx context.Context, cmd *cobra.Command, c *api.ClientWithResponses, v string) (string, error) {
	return resolve(ctx, cmd, folderResource(c), v)
}

func resolveUserRef(ctx context.Context, cmd *cobra.Command, c *api.ClientWithResponses, v string) (string, error) {
	return resolve(ctx, cmd, userResource(c), v)
}
