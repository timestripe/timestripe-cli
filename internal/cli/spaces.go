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
	var (
		f      listFlags
		search string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List spaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			env, err := pagination.Fetch[api.Space](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.Space], error) {
				p := &api.SpacesListParams{
					Limit:  &limit,
					Offset: &offset,
					Search: strFlag(cmd, "search", search),
				}
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
			return renderListOrFail(cmd, env, f.Offset, (&spaceTabular{env.Items}).build())
		},
	}
	addListFlags(cmd, &f)
	cmd.Flags().StringVar(&search, "search", "", "case-insensitive search over name")
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
	var file, name string
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a space",
		Long: "Create a space.\n\n" +
			"The space name may be given as a positional argument or via --name. " +
			"A base JSON body can be loaded with --file (or --file - for stdin); " +
			"any flags passed override the corresponding fields.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := loadBodyFromFile(cmd, file)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				body["name"] = args[0]
			}
			ifChanged(cmd, "name", "name", name, body)

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			ct, r, err := encodeJSONBody(body)
			if err != nil {
				return err
			}
			resp, err := client.SpacesCreateWithBodyWithResponse(cmd.Context(), ct, r)
			if err != nil {
				return err
			}
			if resp.JSON201 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON201, (&spaceTabular{[]api.Space{*resp.JSON201}}).build())
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "JSON body file (or - for stdin); flags override its fields")
	cmd.Flags().StringVar(&name, "name", "", "space name")
	return cmd
}

func newSpacesUpdateCmd() *cobra.Command {
	var file, name string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a space (PATCH)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := loadBodyFromFile(cmd, file)
			if err != nil {
				return err
			}
			ifChanged(cmd, "name", "name", name, body)

			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			ct, r, err := encodeJSONBody(body)
			if err != nil {
				return err
			}
			resp, err := client.SpacesPartialUpdateWithBodyWithResponse(cmd.Context(), args[0], ct, r)
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&spaceTabular{[]api.Space{*resp.JSON200}}).build())
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "JSON body file (or - for stdin); flags override its fields")
	cmd.Flags().StringVar(&name, "name", "", "space name")
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
