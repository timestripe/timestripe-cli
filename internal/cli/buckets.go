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
	var f listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List buckets",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			env, err := pagination.Fetch[api.Bucket](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.Bucket], error) {
				p := &api.BucketsListParams{Limit: &limit, Offset: &offset}
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
			return renderOrFail(cmd, env, (&bucketTabular{env.Items}).build())
		},
	}
	addListFlags(cmd, &f)
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

func newBucketsCreateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a bucket from a JSON body",
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readJSONBody[api.BucketsCreateJSONRequestBody](cmd, file)
			if err != nil {
				return err
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.BucketsCreateWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if resp.JSON201 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON201, (&bucketTabular{[]api.Bucket{*resp.JSON201}}).build())
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to a JSON body (or - for stdin)")
	return cmd
}

func newBucketsUpdateCmd() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a bucket (PATCH)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := readJSONBody[api.BucketsPartialUpdateJSONRequestBody](cmd, file)
			if err != nil {
				return err
			}
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.BucketsPartialUpdateWithResponse(cmd.Context(), args[0], body)
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&bucketTabular{[]api.Bucket{*resp.JSON200}}).build())
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "path to a JSON body (or - for stdin)")
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
