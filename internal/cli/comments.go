package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

func newCommentsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "comments", Short: "Manage comments"}
	cmd.AddCommand(
		newCommentsListCmd(),
		newCommentsGetCmd(),
		newCommentsCreateCmd(),
		newCommentsUpdateCmd(),
		newCommentsDeleteCmd(),
	)
	return cmd
}

func newCommentsListCmd() *cobra.Command {
	var (
		f                                                            listFlags
		goalID, spaceID, userID                                      string
		createdFrom, createdTo, modifiedFrom, modifiedTo, sortFlag   string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List comments",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			cf, err := timeFlag(cmd, "created-from", createdFrom)
			if err != nil {
				return err
			}
			ct, err := timeFlag(cmd, "created-to", createdTo)
			if err != nil {
				return err
			}
			mf, err := timeFlag(cmd, "modified-from", modifiedFrom)
			if err != nil {
				return err
			}
			mt, err := timeFlag(cmd, "modified-to", modifiedTo)
			if err != nil {
				return err
			}
			var sortPtr *api.CommentsListParamsSort
			if cmd.Flags().Changed("sort") {
				s := api.CommentsListParamsSort(sortFlag)
				sortPtr = &s
			}
			env, err := pagination.Fetch[api.Comment](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.Comment], error) {
				p := &api.CommentsListParams{
					Limit:        &limit,
					Offset:       &offset,
					GoalId:       strFlag(cmd, "goal-id", goalID),
					SpaceId:      strFlag(cmd, "space-id", spaceID),
					UserId:       strFlag(cmd, "user-id", userID),
					CreatedFrom:  cf,
					CreatedTo:    ct,
					ModifiedFrom: mf,
					ModifiedTo:   mt,
					Sort:         sortPtr,
				}
				resp, err := client.CommentsListWithResponse(ctx, p)
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Comment]{
					Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous,
					Results: resp.JSON200.Results,
				}, nil
			}, f.options())
			if err != nil {
				return err
			}
			return renderListOrFail(cmd, env, f.Offset, (&commentTabular{env.Items}).build())
		},
	}
	addListFlags(cmd, &f)
	cmd.Flags().StringVar(&goalID, "goal-id", "", "filter by goal ID")
	cmd.Flags().StringVar(&spaceID, "space-id", "", "filter by space ID (via the comment's goal)")
	cmd.Flags().StringVar(&userID, "user-id", "", "filter by author user ID")
	cmd.Flags().StringVar(&createdFrom, "created-from", "", "inclusive lower bound on createdDatetime (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&createdTo, "created-to", "", "exclusive upper bound on createdDatetime (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&modifiedFrom, "modified-from", "", "inclusive lower bound on modifiedDatetime (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&modifiedTo, "modified-to", "", "exclusive upper bound on modifiedDatetime (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&sortFlag, "sort", "", "sort order; prefix with - for descending (e.g. -modifiedDatetime)")
	return cmd
}

func newCommentsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve a comment by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.CommentsRetrieveWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&commentTabular{[]api.Comment{*resp.JSON200}}).build())
		},
	}
}

type commentFields struct {
	file, goalRef, description string
}

func addCommentFields(cmd *cobra.Command, f *commentFields) {
	cmd.Flags().StringVar(&f.file, "file", "", "JSON body file (or - for stdin); flags override its fields")
	cmd.Flags().StringVar(&f.goalRef, "goal", "", "parent goal (ID or name)")
	cmd.Flags().StringVar(&f.description, "description", "", "comment body (Markdown)")
}

func (f *commentFields) build(cmd *cobra.Command, client *api.ClientWithResponses) (map[string]any, error) {
	body, err := loadBodyFromFile(cmd, f.file)
	if err != nil {
		return nil, err
	}
	ifChanged(cmd, "description", "description", f.description, body)
	if cmd.Flags().Changed("goal") {
		id, err := resolveGoalRef(cmd.Context(), client, f.goalRef)
		if err != nil {
			return nil, err
		}
		body["goalId"] = id
	}
	return body, nil
}

func newCommentsCreateCmd() *cobra.Command {
	var f commentFields
	cmd := &cobra.Command{
		Use:   "create [description]",
		Short: "Create a comment",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			body, err := f.build(cmd, client)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				if _, ok := body["description"]; !ok {
					body["description"] = args[0]
				}
			}
			ct, r, err := encodeJSONBody(body)
			if err != nil {
				return err
			}
			resp, err := client.CommentsCreateWithBodyWithResponse(cmd.Context(), ct, r)
			if err != nil {
				return err
			}
			if resp.JSON201 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON201, (&commentTabular{[]api.Comment{*resp.JSON201}}).build())
		},
	}
	addCommentFields(cmd, &f)
	return cmd
}

func newCommentsUpdateCmd() *cobra.Command {
	var f commentFields
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a comment (PATCH)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			body, err := f.build(cmd, client)
			if err != nil {
				return err
			}
			ct, r, err := encodeJSONBody(body)
			if err != nil {
				return err
			}
			resp, err := client.CommentsPartialUpdateWithBodyWithResponse(cmd.Context(), args[0], ct, r)
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&commentTabular{[]api.Comment{*resp.JSON200}}).build())
		},
	}
	addCommentFields(cmd, &f)
	return cmd
}

func newCommentsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a comment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.CommentsDestroyWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
				return apiError(resp.StatusCode(), resp.Body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted comment %s\n", args[0])
			return nil
		},
	}
}

type commentTabular struct{ Comments []api.Comment }

func (c *commentTabular) build() *output.Tabular {
	t := &output.Tabular{Headers: []string{"ID", "GOAL", "USER", "CREATED", "DESCRIPTION"}}
	for _, cm := range c.Comments {
		created := ""
		if cm.CreatedDatetime != nil {
			created = cm.CreatedDatetime.Format("2006-01-02 15:04")
		}
		t.Rows = append(t.Rows, []string{
			ptrStr(cm.Id), cm.GoalId, ptrStr(cm.UserId), created, truncate(cm.Description, 60),
		})
	}
	return t
}

// truncate clips s to n runes, appending an ellipsis if it was shortened.
// Newlines collapse to spaces so table rows stay single-line.
func truncate(s string, n int) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\n' || r == '\r' {
			r = ' '
		}
		out = append(out, r)
	}
	if len(out) <= n {
		return string(out)
	}
	return string(out[:n-1]) + "…"
}
