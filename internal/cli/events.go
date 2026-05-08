package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

func newEventsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "events", Aliases: []string{"event"}, Short: "Inspect activity events (read-only)"}
	cmd.AddCommand(newEventsListCmd(), newEventsGetCmd())
	return cmd
}

func newEventsListCmd() *cobra.Command {
	var (
		f                                          listFlags
		from, to                                   string
		goalID, spaceID, userID, eventType, sortF  string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List events",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			ff, err := timeFlag(cmd, "from", from)
			if err != nil {
				return err
			}
			tt, err := timeFlag(cmd, "to", to)
			if err != nil {
				return err
			}
			var typePtr *api.EventsListParamsType
			if cmd.Flags().Changed("type") {
				t := api.EventsListParamsType(eventType)
				typePtr = &t
			}
			var sortPtr *api.EventsListParamsSort
			if cmd.Flags().Changed("sort") {
				s := api.EventsListParamsSort(sortF)
				sortPtr = &s
			}
			env, err := pagination.Fetch[api.Event](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.Event], error) {
				p := &api.EventsListParams{
					Limit:   &limit,
					Offset:  &offset,
					From:    ff,
					To:      tt,
					GoalId:  strFlag(cmd, "goal-id", goalID),
					SpaceId: strFlag(cmd, "space-id", spaceID),
					UserId:  strFlag(cmd, "user-id", userID),
					Type:    typePtr,
					Sort:    sortPtr,
				}
				resp, err := client.EventsListWithResponse(ctx, p)
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Event]{
					Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous,
					Results: resp.JSON200.Results,
				}, nil
			}, f.options())
			if err != nil {
				return err
			}
			return renderListOrFail(cmd, env, f.Offset, (&eventTabular{env.Items}).build())
		},
	}
	addListFlags(cmd, &f)
	cmd.Flags().StringVar(&from, "from", "", "inclusive lower bound on datetime (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&to, "to", "", "exclusive upper bound on datetime (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&goalID, "goal-id", "", "filter by goal ID")
	cmd.Flags().StringVar(&spaceID, "space-id", "", "filter by space ID")
	cmd.Flags().StringVar(&userID, "user-id", "", "filter by acting user ID")
	cmd.Flags().StringVar(&eventType, "type", "", "filter by event type (e.g. GOAL_CREATED, COMMENT_CREATED, BOARD_MODIFIED)")
	cmd.Flags().StringVar(&sortF, "sort", "", "sort order; prefix with - for descending (e.g. -datetime)")
	return cmd
}

func newEventsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve an event by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.EventsRetrieveWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&eventTabular{[]api.Event{*resp.JSON200}}).build())
		},
	}
}

type eventTabular struct{ Events []api.Event }

func (e *eventTabular) build() *output.Tabular {
	t := &output.Tabular{Headers: []string{"ID", "TYPE", "DATETIME", "USER", "GOAL", "SPACE"}}
	for _, ev := range e.Events {
		when := ""
		if ev.Datetime != nil {
			when = ev.Datetime.Format("2006-01-02 15:04")
		}
		t.Rows = append(t.Rows, []string{
			ptrStr(ev.Id), string(ev.Type), when, ptrStr(ev.UserId), ptrStr(ev.GoalId), ptrStr(ev.SpaceId),
		})
	}
	return t
}
