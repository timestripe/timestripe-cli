package cli

import (
	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
)

// Quick-capture verbs. Each newXxxCmd() builds a fresh command with fresh flag
// variables, so aliasing is just calling the constructor again — no duplicated
// flag definitions, and no drift when the underlying command changes.

func newAddCmd() *cobra.Command {
	c := newGoalsCreateCmd()
	c.Use = "add [name]"
	c.Short = "Create a goal (shorthand for `goals create`)"
	c.GroupID = groupQuick
	return c
}

// newCheckCmd builds `done` / `reopen`, which PATCH a goal's checked state.
func newCheckCmd(use, short string, checked bool) *cobra.Command {
	return &cobra.Command{
		Use:     use + " <goal>",
		Short:   short,
		Args:    cobra.ExactArgs(1),
		GroupID: groupQuick,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			// Accepts a name as well as an ID, so `timestripe done "Buy milk"`
			// works without looking anything up first.
			id, err := resolveGoalRef(cmd.Context(), cmd, client, args[0])
			if err != nil {
				return err
			}
			ct, r, err := encodeJSONBody(map[string]any{"checked": checked})
			if err != nil {
				return err
			}
			resp, err := client.GoalsPartialUpdateWithBodyWithResponse(cmd.Context(), id, ct, r)
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

func newDoneCmd() *cobra.Command {
	return newCheckCmd("done", "Mark a goal done (accepts a name or an ID)", true)
}

func newReopenCmd() *cobra.Command {
	return newCheckCmd("reopen", "Mark a goal not done (accepts a name or an ID)", false)
}

// nested returns a copy for registration under `goals`, where the root help
// groups do not apply.
func nested(c *cobra.Command) *cobra.Command {
	c.GroupID = ""
	return c
}
