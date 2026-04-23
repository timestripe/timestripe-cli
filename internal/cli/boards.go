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
	var (
		f                      listFlags
		spaceID, search, sort  string
		archived               bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List boards",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			var sortPtr *api.BoardsListParamsSort
			if cmd.Flags().Changed("sort") {
				s := api.BoardsListParamsSort(sort)
				sortPtr = &s
			}
			env, err := pagination.Fetch[api.Board](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.Board], error) {
				p := &api.BoardsListParams{
					Limit:    &limit,
					Offset:   &offset,
					SpaceId:  strFlag(cmd, "space-id", spaceID),
					Search:   strFlag(cmd, "search", search),
					Archived: boolFlag(cmd, "archived", archived),
					Sort:     sortPtr,
				}
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
	cmd.Flags().StringVar(&spaceID, "space-id", "", "filter by space ID")
	cmd.Flags().StringVar(&search, "search", "", "case-insensitive search over name")
	cmd.Flags().BoolVar(&archived, "archived", false, "filter by archived state")
	cmd.Flags().StringVar(&sort, "sort", "", "sort order; prefix with - for descending (e.g. -sequenceNo)")
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

type boardFields struct {
	file, name, spaceRef, description, background, layout string
	archived                                              bool
	sequenceNo                                            int
}

func addBoardFields(cmd *cobra.Command, f *boardFields) {
	cmd.Flags().StringVar(&f.file, "file", "", "JSON body file (or - for stdin); flags override its fields")
	cmd.Flags().StringVar(&f.name, "name", "", "board name")
	cmd.Flags().StringVar(&f.spaceRef, "space", "", "parent space (ID or name)")
	cmd.Flags().StringVar(&f.description, "description", "", "board description (Markdown)")
	cmd.Flags().StringVar(&f.background, "background", "", "background (URL or token)")
	cmd.Flags().StringVar(&f.layout, "layout", "", "board layout")
	cmd.Flags().BoolVar(&f.archived, "archived", false, "whether the board is archived")
	cmd.Flags().IntVar(&f.sequenceNo, "sequence-no", 0, "sort order within the parent space")
}

func (f *boardFields) build(cmd *cobra.Command, client *api.ClientWithResponses) (map[string]any, error) {
	body, err := loadBodyFromFile(cmd, f.file)
	if err != nil {
		return nil, err
	}
	ifChanged(cmd, "name", "name", f.name, body)
	ifChanged(cmd, "description", "description", f.description, body)
	ifChanged(cmd, "background", "background", f.background, body)
	ifChanged(cmd, "layout", "layout", f.layout, body)
	ifChanged(cmd, "archived", "archived", f.archived, body)
	ifChanged(cmd, "sequence-no", "sequenceNo", f.sequenceNo, body)
	if cmd.Flags().Changed("space") {
		id, err := resolveSpaceRef(cmd.Context(), client, f.spaceRef)
		if err != nil {
			return nil, err
		}
		body["spaceId"] = id
	}
	return body, nil
}

func newBoardsCreateCmd() *cobra.Command {
	var f boardFields
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a board",
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
			resp, err := client.BoardsCreateWithBodyWithResponse(cmd.Context(), ct, r)
			if err != nil {
				return err
			}
			if resp.JSON201 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON201, (&boardTabular{[]api.Board{*resp.JSON201}}).build())
		},
	}
	addBoardFields(cmd, &f)
	return cmd
}

func newBoardsUpdateCmd() *cobra.Command {
	var f boardFields
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a board (PATCH)",
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
			resp, err := client.BoardsPartialUpdateWithBodyWithResponse(cmd.Context(), args[0], ct, r)
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&boardTabular{[]api.Board{*resp.JSON200}}).build())
		},
	}
	addBoardFields(cmd, &f)
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
			ptrStr(br.Id), ptrStr(br.Name), br.SpaceId, ptrStr((*string)(br.Layout)), ptrBool(br.Archived),
		})
	}
	return t
}
