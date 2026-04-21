package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

func newSpacesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "spaces", Short: "Manage spaces"}
	cmd.AddCommand(
		newSpacesListCmd(),
		newSpacesGetCmd(),
		newSpacesCreateCmd(),
		newSpacesUpdateCmd(),
		newSpacesDeleteCmd(),
	)
	return cmd
}

func newSpacesListCmd() *cobra.Command {
	var f listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List spaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			env, err := pagination.Fetch[api.Space](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.Space], error) {
				p := &api.SpacesListParams{Limit: &limit, Offset: &offset}
				resp, err := client.SpacesListWithResponse(ctx, p)
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Space]{
					Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous,
					Results: resp.JSON200.Results,
				}, nil
			}, f.options())
			if err != nil {
				return err
			}
			return renderOrFail(cmd, env, (&spaceTabular{env.Items}).build())
		},
	}
	addListFlags(cmd, &f)
	return cmd
}

func newSpacesGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve a space by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.SpacesRetrieveWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&spaceTabular{[]api.Space{*resp.JSON200}}).build())
		},
	}
}

func newSpacesCreateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a space from a JSON body",
		Long:  "Reads a JSON body from --file <path>, or --file - for stdin.",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readJSONBody[api.SpacesCreateJSONRequestBody](cmd, file)
			if err != nil {
				return err
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.SpacesCreateWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if resp.JSON201 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON201, (&spaceTabular{[]api.Space{*resp.JSON201}}).build())
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to a JSON body (or - for stdin)")
	return cmd
}

func newSpacesUpdateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a space from a JSON body (PATCH)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readJSONBody[api.SpacesPartialUpdateJSONRequestBody](cmd, file)
			if err != nil {
				return err
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.SpacesPartialUpdateWithResponse(cmd.Context(), args[0], body)
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&spaceTabular{[]api.Space{*resp.JSON200}}).build())
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to a JSON body (or - for stdin)")
	return cmd
}

func newSpacesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a space",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.SpacesDestroyWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
				return apiError(resp.StatusCode(), resp.Body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted space %s\n", args[0])
			return nil
		},
	}
}

type spaceTabular struct{ Spaces []api.Space }

func (s *spaceTabular) build() *output.Tabular {
	t := &output.Tabular{Headers: []string{"ID", "NAME", "URL"}}
	for _, sp := range s.Spaces {
		t.Rows = append(t.Rows, []string{ptrStr(sp.Id), ptrStr(sp.Name), ptrStr(sp.Url)})
	}
	return t
}
