package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

func newBucketsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "buckets", Short: "Manage buckets"}
	cmd.AddCommand(
		newBucketsListCmd(),
		newBucketsGetCmd(),
		newBucketsCreateCmd(),
		newBucketsUpdateCmd(),
		newBucketsDeleteCmd(),
	)
	return cmd
}

func newBucketsListCmd() *cobra.Command {
	var (
		f                     listFlags
		boardID, search, sort string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List buckets",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			var sortPtr *api.BucketsListParamsSort
			if cmd.Flags().Changed("sort") {
				s := api.BucketsListParamsSort(sort)
				sortPtr = &s
			}
			env, err := pagination.Fetch[api.Bucket](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.Bucket], error) {
				p := &api.BucketsListParams{
					Limit:   &limit,
					Offset:  &offset,
					BoardId: strFlag(cmd, "board-id", boardID),
					Search:  strFlag(cmd, "search", search),
					Sort:    sortPtr,
				}
				resp, err := client.BucketsListWithResponse(ctx, p)
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Bucket]{
					Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous,
					Results: resp.JSON200.Results,
				}, nil
			}, f.options())
			if err != nil {
				return err
			}
			return renderListOrFail(cmd, env, f.Offset, (&bucketTabular{env.Items}).build())
		},
	}
	addListFlags(cmd, &f)
	cmd.Flags().StringVar(&boardID, "board-id", "", "filter by board ID")
	cmd.Flags().StringVar(&search, "search", "", "case-insensitive search over name")
	cmd.Flags().StringVar(&sort, "sort", "", "sort order; prefix with - for descending (e.g. -sequenceNo)")
	return cmd
}

func newBucketsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve a bucket by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.BucketsRetrieveWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&bucketTabular{[]api.Bucket{*resp.JSON200}}).build())
		},
	}
}

type bucketFields struct {
	file, name, boardRef, emoji string
	expanded, showEmoji         bool
	sequenceNo                  int
}

func addBucketFields(cmd *cobra.Command, f *bucketFields) {
	cmd.Flags().StringVar(&f.file, "file", "", "JSON body file (or - for stdin); flags override its fields")
	cmd.Flags().StringVar(&f.name, "name", "", "bucket name")
	cmd.Flags().StringVar(&f.boardRef, "board", "", "parent board (ID or name)")
	cmd.Flags().StringVar(&f.emoji, "emoji", "", "display emoji")
	cmd.Flags().BoolVar(&f.expanded, "expanded", false, "whether the bucket is shown expanded")
	cmd.Flags().BoolVar(&f.showEmoji, "show-emoji", false, "whether to show the emoji")
	cmd.Flags().IntVar(&f.sequenceNo, "sequence-no", 0, "sort order within the parent board")
}

func (f *bucketFields) build(cmd *cobra.Command, client *api.ClientWithResponses) (map[string]any, error) {
	body, err := loadBodyFromFile(cmd, f.file)
	if err != nil {
		return nil, err
	}
	ifChanged(cmd, "name", "name", f.name, body)
	ifChanged(cmd, "emoji", "emoji", f.emoji, body)
	ifChanged(cmd, "expanded", "expanded", f.expanded, body)
	ifChanged(cmd, "show-emoji", "showEmoji", f.showEmoji, body)
	ifChanged(cmd, "sequence-no", "sequenceNo", f.sequenceNo, body)
	if cmd.Flags().Changed("board") {
		id, err := resolveBoardRef(cmd.Context(), client, f.boardRef)
		if err != nil {
			return nil, err
		}
		body["boardId"] = id
	}
	return body, nil
}

func newBucketsCreateCmd() *cobra.Command {
	var f bucketFields
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a bucket",
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
			resp, err := client.BucketsCreateWithBodyWithResponse(cmd.Context(), ct, r)
			if err != nil {
				return err
			}
			if resp.JSON201 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON201, (&bucketTabular{[]api.Bucket{*resp.JSON201}}).build())
		},
	}
	addBucketFields(cmd, &f)
	return cmd
}

func newBucketsUpdateCmd() *cobra.Command {
	var f bucketFields
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a bucket (PATCH)",
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
			resp, err := client.BucketsPartialUpdateWithBodyWithResponse(cmd.Context(), args[0], ct, r)
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&bucketTabular{[]api.Bucket{*resp.JSON200}}).build())
		},
	}
	addBucketFields(cmd, &f)
	return cmd
}

func newBucketsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a bucket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.BucketsDestroyWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
				return apiError(resp.StatusCode(), resp.Body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted bucket %s\n", args[0])
			return nil
		},
	}
}

type bucketTabular struct{ Buckets []api.Bucket }

func (b *bucketTabular) build() *output.Tabular {
	t := &output.Tabular{Headers: []string{"ID", "NAME", "BOARD", "SEQUENCE"}}
	for _, bk := range b.Buckets {
		t.Rows = append(t.Rows, []string{
			ptrStr(bk.Id), ptrStr(bk.Name), bk.BoardId, ptrInt(bk.SequenceNo),
		})
	}
	return t
}
