package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

func newMembershipsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "memberships", Short: "Manage memberships (read-only)"}
	cmd.AddCommand(newMembershipsListCmd(), newMembershipsGetCmd())
	return cmd
}

func newMembershipsListCmd() *cobra.Command {
	var (
		f                listFlags
		spaceID, userID  string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List memberships",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			env, err := pagination.Fetch[api.Membership](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.Membership], error) {
				p := &api.MembershipsListParams{
					Limit:   &limit,
					Offset:  &offset,
					SpaceId: strFlag(cmd, "space-id", spaceID),
					UserId:  strFlag(cmd, "user-id", userID),
				}
				resp, err := client.MembershipsListWithResponse(ctx, p)
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Membership]{
					Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous,
					Results: resp.JSON200.Results,
				}, nil
			}, f.options())
			if err != nil {
				return err
			}
			return renderListOrFail(cmd, env, f.Offset, (&membershipTabular{env.Items}).build())
		},
	}
	addListFlags(cmd, &f)
	cmd.Flags().StringVar(&spaceID, "space-id", "", "filter by space ID")
	cmd.Flags().StringVar(&userID, "user-id", "", "filter by user ID")
	return cmd
}

func newMembershipsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve a membership by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.MembershipsRetrieveWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&membershipTabular{[]api.Membership{*resp.JSON200}}).build())
		},
	}
}

type membershipTabular struct{ Memberships []api.Membership }

func (m *membershipTabular) build() *output.Tabular {
	t := &output.Tabular{Headers: []string{"ID", "SPACE", "USER", "ROLE"}}
	for _, mb := range m.Memberships {
		role := ""
		if mb.Role != nil {
			role = string(*mb.Role)
		}
		t.Rows = append(t.Rows, []string{
			ptrStr(mb.Id), ptrStr(mb.SpaceId), ptrStr(mb.UserId), role,
		})
	}
	return t
}
