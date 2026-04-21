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

func newGoalsCreateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a goal from a JSON body",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readJSONBody[api.GoalsCreateJSONRequestBody](cmd, file)
			if err != nil {
				return err
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.GoalsCreateWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if resp.JSON201 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON201, (&goalTabular{[]api.Goal{*resp.JSON201}}).build())
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to a JSON body (or - for stdin)")
	return cmd
}

func newGoalsUpdateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a goal (PATCH)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readJSONBody[api.GoalsPartialUpdateJSONRequestBody](cmd, file)
			if err != nil {
				return err
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.GoalsPartialUpdateWithResponse(cmd.Context(), args[0], body)
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&goalTabular{[]api.Goal{*resp.JSON200}}).build())
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to a JSON body (or - for stdin)")
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
