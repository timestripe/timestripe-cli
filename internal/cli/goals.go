package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

func newGoalsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "goals", Short: "Manage goals"}
	cmd.AddCommand(
		newGoalsListCmd(),
		newGoalsGetCmd(),
		newGoalsCreateCmd(),
		newGoalsUpdateCmd(),
		newGoalsDeleteCmd(),
	)
	return cmd
}

func newGoalsListCmd() *cobra.Command {
	var f listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List goals",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			env, err := pagination.Fetch[api.Goal](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.Goal], error) {
				p := &api.GoalsListParams{Limit: &limit, Offset: &offset}
				resp, err := client.GoalsListWithResponse(ctx, p)
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Goal]{
					Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous,
					Results: resp.JSON200.Results,
				}, nil
			}, f.options())
			if err != nil {
				return err
			}
			return renderOrFail(cmd, env, (&goalTabular{env.Items}).build())
		},
	}
	addListFlags(cmd, &f)
	return cmd
}

func newGoalsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve a goal by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.GoalsRetrieveWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&goalTabular{[]api.Goal{*resp.JSON200}}).build())
		},
	}
}

type goalFields struct {
	file, name                                            string
	spaceRef, bucketRef, assigneeRef, parentRef           string
	description, horizon, color, date, startTime, endTime string
	checked                                               bool
}

func addGoalFields(cmd *cobra.Command, f *goalFields) {
	cmd.Flags().StringVar(&f.file, "file", "", "JSON body file (or - for stdin); flags override its fields")
	cmd.Flags().StringVar(&f.name, "name", "", "goal name")
	cmd.Flags().StringVar(&f.spaceRef, "space", "", "parent space (ID or name)")
	cmd.Flags().StringVar(&f.bucketRef, "bucket", "", "parent bucket (ID or name)")
	cmd.Flags().StringVar(&f.assigneeRef, "assignee", "", "assignee (user ID, email, or full name)")
	cmd.Flags().StringVar(&f.parentRef, "parent", "", "parent goal (ID or name)")
	cmd.Flags().StringVar(&f.description, "description", "", "goal description")
	cmd.Flags().StringVar(&f.horizon, "horizon", "", "horizon: day|week|month|quarter|year|decade|life")
	cmd.Flags().StringVar(&f.color, "color", "", "palette color (e.g. #ecce32)")
	cmd.Flags().StringVar(&f.date, "date", "", "ISO date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&f.startTime, "start-time", "", "start time")
	cmd.Flags().StringVar(&f.endTime, "end-time", "", "end time")
	cmd.Flags().BoolVar(&f.checked, "checked", false, "whether the goal is checked/done")
}

func (f *goalFields) build(cmd *cobra.Command, client *api.ClientWithResponses) (map[string]any, error) {
	body, err := loadBodyFromFile(cmd, f.file)
	if err != nil {
		return nil, err
	}
	ifChanged(cmd, "name", "name", f.name, body)
	ifChanged(cmd, "description", "description", f.description, body)
	ifChanged(cmd, "horizon", "horizon", f.horizon, body)
	ifChanged(cmd, "color", "color", f.color, body)
	ifChanged(cmd, "date", "date", f.date, body)
	ifChanged(cmd, "start-time", "startTime", f.startTime, body)
	ifChanged(cmd, "end-time", "endTime", f.endTime, body)
	ifChanged(cmd, "checked", "checked", f.checked, body)
	if cmd.Flags().Changed("space") {
		id, err := resolveSpaceRef(cmd.Context(), client, f.spaceRef)
		if err != nil {
			return nil, err
		}
		body["spaceId"] = id
	}
	if cmd.Flags().Changed("bucket") {
		id, err := resolveBucketRef(cmd.Context(), client, f.bucketRef)
		if err != nil {
			return nil, err
		}
		body["bucketId"] = id
	}
	if cmd.Flags().Changed("assignee") {
		id, err := resolveUserRef(cmd.Context(), client, f.assigneeRef)
		if err != nil {
			return nil, err
		}
		body["assigneeId"] = id
	}
	if cmd.Flags().Changed("parent") {
		id, err := resolveGoalRef(cmd.Context(), client, f.parentRef)
		if err != nil {
			return nil, err
		}
		body["parentId"] = id
	}
	return body, nil
}

func newGoalsCreateCmd() *cobra.Command {
	var f goalFields
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a goal",
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
				body["name"] = args[0]
			}
			ct, r, err := encodeJSONBody(body)
			if err != nil {
				return err
			}
			resp, err := client.GoalsCreateWithBodyWithResponse(cmd.Context(), ct, r)
			if err != nil {
				return err
			}
			if resp.JSON201 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON201, (&goalTabular{[]api.Goal{*resp.JSON201}}).build())
		},
	}
	addGoalFields(cmd, &f)
	return cmd
}

func newGoalsUpdateCmd() *cobra.Command {
	var f goalFields
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a goal (PATCH)",
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
			resp, err := client.GoalsPartialUpdateWithBodyWithResponse(cmd.Context(), args[0], ct, r)
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&goalTabular{[]api.Goal{*resp.JSON200}}).build())
		},
	}
	addGoalFields(cmd, &f)
	return cmd
}

func newGoalsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a goal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.GoalsDestroyWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
				return apiError(resp.StatusCode(), resp.Body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted goal %s\n", args[0])
			return nil
		},
	}
}

type goalTabular struct{ Goals []api.Goal }

func (g *goalTabular) build() *output.Tabular {
	t := &output.Tabular{Headers: []string{"ID", "NAME", "HORIZON", "DATE", "CHECKED", "SPACE"}}
	for _, gl := range g.Goals {
		horizon := ""
		if gl.Horizon != nil {
			horizon = string(*gl.Horizon)
		}
		date := ""
		if gl.Date != nil {
			date = gl.Date.Time.Format("2006-01-02")
		}
		t.Rows = append(t.Rows, []string{
			ptrStr(gl.Id), ptrStr(gl.Name), horizon, date, ptrBool(gl.Checked), gl.SpaceId,
		})
	}
	return t
}
