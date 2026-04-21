package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

func newBoardsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "boards", Short: "Manage boards"}
	cmd.AddCommand(
		newBoardsListCmd(),
		newBoardsGetCmd(),
		newBoardsCreateCmd(),
		newBoardsUpdateCmd(),
		newBoardsDeleteCmd(),
	)
	return cmd
}

func newBoardsListCmd() *cobra.Command {
	var f listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List boards",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			env, err := pagination.Fetch[api.Board](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.Board], error) {
				p := &api.BoardsListParams{Limit: &limit, Offset: &offset}
				resp, err := client.BoardsListWithResponse(ctx, p)
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Board]{
					Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous,
					Results: resp.JSON200.Results,
				}, nil
			}, f.options())
			if err != nil {
				return err
			}
			return renderOrFail(cmd, env, (&boardTabular{env.Items}).build())
		},
	}
	addListFlags(cmd, &f)
	return cmd
}

func newBoardsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve a board by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.BoardsRetrieveWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&boardTabular{[]api.Board{*resp.JSON200}}).build())
		},
	}
}

func newBoardsCreateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a board from a JSON body",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readJSONBody[api.BoardsCreateJSONRequestBody](cmd, file)
			if err != nil {
				return err
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.BoardsCreateWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if resp.JSON201 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON201, (&boardTabular{[]api.Board{*resp.JSON201}}).build())
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to a JSON body (or - for stdin)")
	return cmd
}

func newBoardsUpdateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a board (PATCH)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readJSONBody[api.BoardsPartialUpdateJSONRequestBody](cmd, file)
			if err != nil {
				return err
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.BoardsPartialUpdateWithResponse(cmd.Context(), args[0], body)
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&boardTabular{[]api.Board{*resp.JSON200}}).build())
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to a JSON body (or - for stdin)")
	return cmd
}

func newBoardsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a board",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.BoardsDestroyWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
				return apiError(resp.StatusCode(), resp.Body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted board %s\n", args[0])
			return nil
		},
	}
}

type boardTabular struct{ Boards []api.Board }

func (b *boardTabular) build() *output.Tabular {
	t := &output.Tabular{Headers: []string{"ID", "NAME", "SPACE", "LAYOUT", "ARCHIVED"}}
	for _, br := range b.Boards {
		t.Rows = append(t.Rows, []string{
			ptrStr(br.Id), ptrStr(br.Name), br.SpaceId, ptrStr(br.Layout), ptrBool(br.Archived),
		})
	}
	return t
}
