package cli

import (
	"context"
	"fmt"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

// resolveRef turns a user-supplied value into a canonical resource ID.
//
// Strategy (matches the "prefer id on conflict" requirement):
//  1. Try a direct GET by ID. If the server returns 2xx, use that ID.
//  2. Otherwise, list every accessible resource of that type and match by
//     the display name. Ambiguous name matches return an error.
//
// The first step is what makes ID win over a conflicting name: if the value
// happens to also be some other resource's name, the GET short-circuits and
// the name lookup never runs.
func resolveRef[T any](
	ctx context.Context,
	value string,
	getByID func(ctx context.Context, id string) (*T, error),
	listAll func(ctx context.Context) ([]T, error),
	idOf func(T) string,
	nameOf func(T) string,
	kind string,
) (string, error) {
	if value == "" {
		return "", fmt.Errorf("empty %s reference", kind)
	}
	if v, err := getByID(ctx, value); err == nil && v != nil {
		return idOf(*v), nil
	}
	items, err := listAll(ctx)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, it := range items {
		if nameOf(it) == value {
			matches = append(matches, idOf(it))
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no %s matches %q (not a valid ID and no name match)", kind, value)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("%q matches %d %ss by name; disambiguate with an ID", value, len(matches), kind)
	}
}

// Concrete resolvers per resource. Each fetches the whole list via --all
// semantics when falling back to a name match.

func resolveSpaceRef(ctx context.Context, c *api.ClientWithResponses, value string) (string, error) {
	return resolveRef(ctx, value,
		func(ctx context.Context, id string) (*api.Space, error) {
			resp, err := c.SpacesRetrieveWithResponse(ctx, id)
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return resp.JSON200, nil
		},
		func(ctx context.Context) ([]api.Space, error) {
			env, err := pagination.Fetch[api.Space](ctx, func(ctx context.Context, limit, offset int) (*pagination.Page[api.Space], error) {
				resp, err := c.SpacesListWithResponse(ctx, &api.SpacesListParams{Limit: &limit, Offset: &offset})
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Space]{Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous, Results: resp.JSON200.Results}, nil
			}, pagination.Options{All: true})
			if err != nil {
				return nil, err
			}
			return env.Items, nil
		},
		func(s api.Space) string { return ptrStr(s.Id) },
		func(s api.Space) string { return ptrStr(s.Name) },
		"space",
	)
}

func resolveBoardRef(ctx context.Context, c *api.ClientWithResponses, value string) (string, error) {
	return resolveRef(ctx, value,
		func(ctx context.Context, id string) (*api.Board, error) {
			resp, err := c.BoardsRetrieveWithResponse(ctx, id)
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return resp.JSON200, nil
		},
		func(ctx context.Context) ([]api.Board, error) {
			env, err := pagination.Fetch[api.Board](ctx, func(ctx context.Context, limit, offset int) (*pagination.Page[api.Board], error) {
				resp, err := c.BoardsListWithResponse(ctx, &api.BoardsListParams{Limit: &limit, Offset: &offset})
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Board]{Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous, Results: resp.JSON200.Results}, nil
			}, pagination.Options{All: true})
			if err != nil {
				return nil, err
			}
			return env.Items, nil
		},
		func(b api.Board) string { return ptrStr(b.Id) },
		func(b api.Board) string { return ptrStr(b.Name) },
		"board",
	)
}

func resolveBucketRef(ctx context.Context, c *api.ClientWithResponses, value string) (string, error) {
	return resolveRef(ctx, value,
		func(ctx context.Context, id string) (*api.Bucket, error) {
			resp, err := c.BucketsRetrieveWithResponse(ctx, id)
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return resp.JSON200, nil
		},
		func(ctx context.Context) ([]api.Bucket, error) {
			env, err := pagination.Fetch[api.Bucket](ctx, func(ctx context.Context, limit, offset int) (*pagination.Page[api.Bucket], error) {
				resp, err := c.BucketsListWithResponse(ctx, &api.BucketsListParams{Limit: &limit, Offset: &offset})
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Bucket]{Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous, Results: resp.JSON200.Results}, nil
			}, pagination.Options{All: true})
			if err != nil {
				return nil, err
			}
			return env.Items, nil
		},
		func(b api.Bucket) string { return ptrStr(b.Id) },
		func(b api.Bucket) string { return ptrStr(b.Name) },
		"bucket",
	)
}

func resolveGoalRef(ctx context.Context, c *api.ClientWithResponses, value string) (string, error) {
	return resolveRef(ctx, value,
		func(ctx context.Context, id string) (*api.Goal, error) {
			resp, err := c.GoalsRetrieveWithResponse(ctx, id)
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return resp.JSON200, nil
		},
		func(ctx context.Context) ([]api.Goal, error) {
			env, err := pagination.Fetch[api.Goal](ctx, func(ctx context.Context, limit, offset int) (*pagination.Page[api.Goal], error) {
				resp, err := c.GoalsListWithResponse(ctx, &api.GoalsListParams{Limit: &limit, Offset: &offset})
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Goal]{Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous, Results: resp.JSON200.Results}, nil
			}, pagination.Options{All: true})
			if err != nil {
				return nil, err
			}
			return env.Items, nil
		},
		func(g api.Goal) string { return ptrStr(g.Id) },
		func(g api.Goal) string { return ptrStr(g.Name) },
		"goal",
	)
}

// resolveUserRef matches against email, then "First Last" full name.
func resolveUserRef(ctx context.Context, c *api.ClientWithResponses, value string) (string, error) {
	return resolveRef(ctx, value,
		func(ctx context.Context, id string) (*api.User, error) {
			resp, err := c.UsersRetrieveWithResponse(ctx, id)
			if err != nil {
				return nil, err
			}
			if resp.JSON200 == nil {
				return nil, apiError(resp.StatusCode(), resp.Body)
			}
			return resp.JSON200, nil
		},
		func(ctx context.Context) ([]api.User, error) {
			env, err := pagination.Fetch[api.User](ctx, func(ctx context.Context, limit, offset int) (*pagination.Page[api.User], error) {
				resp, err := c.UsersListWithResponse(ctx, &api.UsersListParams{Limit: &limit, Offset: &offset})
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.User]{Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous, Results: resp.JSON200.Results}, nil
			}, pagination.Options{All: true})
			if err != nil {
				return nil, err
			}
			return env.Items, nil
		},
		func(u api.User) string { return ptrStr(u.Id) },
		func(u api.User) string {
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
		},
		"user",
	)
}

