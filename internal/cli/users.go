package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

func newUsersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "users", Short: "Manage users (read-only)"}
	cmd.AddCommand(newUsersListCmd(), newUsersGetCmd(), newUsersMeCmd())
	return cmd
}

func newUsersMeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Show the currently authenticated user",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.UsersMeRetrieveWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			tab := (&userTabular{Users: []api.User{*resp.JSON200}}).build()
			return renderOrFail(cmd, resp.JSON200, tab)
		},
	}
}

func newUsersListCmd() *cobra.Command {
	var (
		f             listFlags
		email, search string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			env, err := pagination.Fetch[api.User](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.User], error) {
				p := &api.UsersListParams{
					Limit:  &limit,
					Offset: &offset,
					Email:  strFlag(cmd, "email", email),
					Search: strFlag(cmd, "search", search),
				}
				resp, err := client.UsersListWithResponse(ctx, p)
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.User]{
					Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous,
					Results: resp.JSON200.Results,
				}, nil
			}, f.options())
			if err != nil {
				return err
			}
			tab := (&userTabular{Users: env.Items}).build()
			return renderOrFail(cmd, env, tab)
		},
	}
	addListFlags(cmd, &f)
	cmd.Flags().StringVar(&email, "email", "", "filter by exact email match")
	cmd.Flags().StringVar(&search, "search", "", "case-insensitive search over first name, last name, email")
	return cmd
}

func newUsersGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve a user by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.UsersRetrieveWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			tab := (&userTabular{Users: []api.User{*resp.JSON200}}).build()
			return renderOrFail(cmd, resp.JSON200, tab)
		},
	}
}

type userTabular struct{ Users []api.User }

func (u *userTabular) build() *output.Tabular {
	t := &output.Tabular{Headers: []string{"ID", "EMAIL", "NAME", "TIMEZONE"}}
	for _, user := range u.Users {
		email := ""
		if user.Email != nil {
			email = string(*user.Email)
		}
		name := ptrStr(user.FirstName)
		if last := ptrStr(user.LastName); last != "" {
			if name == "" {
				name = last
			} else {
				name = name + " " + last
			}
		}
		t.Rows = append(t.Rows, []string{
			ptrStr(user.Id), email, name, ptrStr(user.Timezone),
		})
	}
	return t
}
