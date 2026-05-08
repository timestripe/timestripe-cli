package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/timestripe/timestripe-cli/internal/api"
	"github.com/timestripe/timestripe-cli/internal/output"
	"github.com/timestripe/timestripe-cli/internal/pagination"
)

func newFoldersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "folders", Aliases: []string{"folder"}, Short: "Manage folders"}
	cmd.AddCommand(
		newFoldersListCmd(),
		newFoldersGetCmd(),
		newFoldersCreateCmd(),
		newFoldersUpdateCmd(),
		newFoldersDeleteCmd(),
		newFolderGoalsCmd(),
	)
	return cmd
}

func newFoldersListCmd() *cobra.Command {
	var (
		f                       listFlags
		spaceID, userID, sortF  string
		isPrivate               bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List folders",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			var sortPtr *api.FoldersListParamsSort
			if cmd.Flags().Changed("sort") {
				s := api.FoldersListParamsSort(sortF)
				sortPtr = &s
			}
			env, err := pagination.Fetch[api.Folder](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.Folder], error) {
				p := &api.FoldersListParams{
					Limit:     &limit,
					Offset:    &offset,
					SpaceId:   strFlag(cmd, "space-id", spaceID),
					UserId:    strFlag(cmd, "user-id", userID),
					IsPrivate: boolFlag(cmd, "is-private", isPrivate),
					Sort:      sortPtr,
				}
				resp, err := client.FoldersListWithResponse(ctx, p)
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.Folder]{
					Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous,
					Results: resp.JSON200.Results,
				}, nil
			}, f.options())
			if err != nil {
				return err
			}
			return renderListOrFail(cmd, env, f.Offset, (&folderTabular{env.Items}).build())
		},
	}
	addListFlags(cmd, &f)
	cmd.Flags().StringVar(&spaceID, "space-id", "", "filter by space ID")
	cmd.Flags().StringVar(&userID, "user-id", "", "filter by owning user ID (pass \"null\" for shared folders)")
	cmd.Flags().BoolVar(&isPrivate, "is-private", false, "filter by private state")
	cmd.Flags().StringVar(&sortF, "sort", "", "sort order; prefix with - for descending (e.g. -sequence_no)")
	return cmd
}

func newFoldersGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve a folder by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.FoldersRetrieveWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&folderTabular{[]api.Folder{*resp.JSON200}}).build())
		},
	}
}

type folderFields struct {
	file, name, spaceRef string
	sequenceNo           int
	isPrivate            bool
}

func addFolderFields(cmd *cobra.Command, f *folderFields) {
	cmd.Flags().StringVar(&f.file, "file", "", "JSON body file (or - for stdin); flags override its fields")
	cmd.Flags().StringVar(&f.name, "name", "", "folder name")
	cmd.Flags().StringVar(&f.spaceRef, "space", "", "parent space (ID or name)")
	cmd.Flags().IntVar(&f.sequenceNo, "sequence-no", 0, "sort order within the parent space")
	cmd.Flags().BoolVar(&f.isPrivate, "is-private", false, "whether the folder is private to its owner")
}

func (f *folderFields) build(cmd *cobra.Command, client *api.ClientWithResponses) (map[string]any, error) {
	body, err := loadBodyFromFile(cmd, f.file)
	if err != nil {
		return nil, err
	}
	ifChanged(cmd, "name", "name", f.name, body)
	ifChanged(cmd, "sequence-no", "sequence_no", f.sequenceNo, body)
	ifChanged(cmd, "is-private", "is_private", f.isPrivate, body)
	if cmd.Flags().Changed("space") {
		id, err := resolveSpaceRef(cmd.Context(), client, f.spaceRef)
		if err != nil {
			return nil, err
		}
		body["space_id"] = id
	}
	return body, nil
}

func newFoldersCreateCmd() *cobra.Command {
	var f folderFields
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a folder",
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
			resp, err := client.FoldersCreateWithBodyWithResponse(cmd.Context(), ct, r)
			if err != nil {
				return err
			}
			if resp.JSON201 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON201, (&folderTabular{[]api.Folder{*resp.JSON201}}).build())
		},
	}
	addFolderFields(cmd, &f)
	return cmd
}

func newFoldersUpdateCmd() *cobra.Command {
	var f folderFields
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a folder (PATCH)",
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
			resp, err := client.FoldersPartialUpdateWithBodyWithResponse(cmd.Context(), args[0], ct, r)
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&folderTabular{[]api.Folder{*resp.JSON200}}).build())
		},
	}
	addFolderFields(cmd, &f)
	return cmd
}

func newFoldersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.FoldersDestroyWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
				return apiError(resp.StatusCode(), resp.Body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted folder %s\n", args[0])
			return nil
		},
	}
}

type folderTabular struct{ Folders []api.Folder }

func (b *folderTabular) build() *output.Tabular {
	t := &output.Tabular{Headers: []string{"ID", "NAME", "SPACE", "PRIVATE", "SEQUENCE"}}
	for _, fl := range b.Folders {
		t.Rows = append(t.Rows, []string{
			ptrStr(fl.Id), ptrStr(fl.Name), fl.SpaceId, ptrBool(fl.IsPrivate), ptrInt(fl.SequenceNo),
		})
	}
	return t
}

// folder-goals (membership) subtree

func newFolderGoalsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "goals",
		Aliases: []string{"goal", "tasks", "task", "todos", "todo", "items", "item"},
		Short:   "Manage which goals belong to folders",
	}
	cmd.AddCommand(
		newFolderGoalsListCmd(),
		newFolderGoalsGetCmd(),
		newFolderGoalsAddCmd(),
		newFolderGoalsUpdateCmd(),
		newFolderGoalsRemoveCmd(),
	)
	return cmd
}

func newFolderGoalsListCmd() *cobra.Command {
	var (
		f                      listFlags
		folderID, goalID, sortF string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List folder-goal memberships",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			var sortPtr *api.FolderGoalsListParamsSort
			if cmd.Flags().Changed("sort") {
				s := api.FolderGoalsListParamsSort(sortF)
				sortPtr = &s
			}
			env, err := pagination.Fetch[api.FolderGoal](cmd.Context(), func(ctx context.Context, limit, offset int) (*pagination.Page[api.FolderGoal], error) {
				p := &api.FolderGoalsListParams{
					Limit:    &limit,
					Offset:   &offset,
					FolderId: strFlag(cmd, "folder-id", folderID),
					GoalId:   strFlag(cmd, "goal-id", goalID),
					Sort:     sortPtr,
				}
				resp, err := client.FolderGoalsListWithResponse(ctx, p)
				if err != nil {
					return nil, err
				}
				if resp.JSON200 == nil {
					return nil, apiError(resp.StatusCode(), resp.Body)
				}
				return &pagination.Page[api.FolderGoal]{
					Count: resp.JSON200.Count, Next: resp.JSON200.Next, Previous: resp.JSON200.Previous,
					Results: resp.JSON200.Results,
				}, nil
			}, f.options())
			if err != nil {
				return err
			}
			return renderListOrFail(cmd, env, f.Offset, (&folderGoalTabular{env.Items}).build())
		},
	}
	addListFlags(cmd, &f)
	cmd.Flags().StringVar(&folderID, "folder-id", "", "filter by folder ID")
	cmd.Flags().StringVar(&goalID, "goal-id", "", "filter by goal ID")
	cmd.Flags().StringVar(&sortF, "sort", "", "sort order; prefix with - for descending (e.g. -sequence_no)")
	return cmd
}

func newFolderGoalsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve a folder-goal membership by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.FolderGoalsRetrieveWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&folderGoalTabular{[]api.FolderGoal{*resp.JSON200}}).build())
		},
	}
}

type folderGoalFields struct {
	file, folderRef, goalRef string
	sequenceNo               int
}

func addFolderGoalFields(cmd *cobra.Command, f *folderGoalFields) {
	cmd.Flags().StringVar(&f.file, "file", "", "JSON body file (or - for stdin); flags override its fields")
	cmd.Flags().StringVar(&f.folderRef, "folder", "", "parent folder (ID or name)")
	cmd.Flags().StringVar(&f.goalRef, "goal", "", "linked goal (ID or name)")
	cmd.Flags().IntVar(&f.sequenceNo, "sequence-no", 0, "sort order within the folder")
}

func (f *folderGoalFields) build(cmd *cobra.Command, client *api.ClientWithResponses) (map[string]any, error) {
	body, err := loadBodyFromFile(cmd, f.file)
	if err != nil {
		return nil, err
	}
	ifChanged(cmd, "sequence-no", "sequence_no", f.sequenceNo, body)
	if cmd.Flags().Changed("folder") {
		id, err := resolveFolderRef(cmd.Context(), client, f.folderRef)
		if err != nil {
			return nil, err
		}
		body["folder_id"] = id
	}
	if cmd.Flags().Changed("goal") {
		id, err := resolveGoalRef(cmd.Context(), client, f.goalRef)
		if err != nil {
			return nil, err
		}
		body["goal_id"] = id
	}
	return body, nil
}

func newFolderGoalsAddCmd() *cobra.Command {
	var f folderGoalFields
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a goal to a folder",
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
			resp, err := client.FolderGoalsCreateWithBodyWithResponse(cmd.Context(), ct, r)
			if err != nil {
				return err
			}
			if resp.JSON201 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON201, (&folderGoalTabular{[]api.FolderGoal{*resp.JSON201}}).build())
		},
	}
	addFolderGoalFields(cmd, &f)
	return cmd
}

func newFolderGoalsUpdateCmd() *cobra.Command {
	var f folderGoalFields
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Partially update a folder-goal membership (PATCH)",
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
			resp, err := client.FolderGoalsPartialUpdateWithBodyWithResponse(cmd.Context(), args[0], ct, r)
			if err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return apiError(resp.StatusCode(), resp.Body)
			}
			return renderOrFail(cmd, resp.JSON200, (&folderGoalTabular{[]api.FolderGoal{*resp.JSON200}}).build())
		},
	}
	addFolderGoalFields(cmd, &f)
	return cmd
}

func newFolderGoalsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a goal from a folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}
			resp, err := client.FolderGoalsDestroyWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
				return apiError(resp.StatusCode(), resp.Body)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed folder-goal %s\n", args[0])
			return nil
		},
	}
}

type folderGoalTabular struct{ Items []api.FolderGoal }

func (g *folderGoalTabular) build() *output.Tabular {
	t := &output.Tabular{Headers: []string{"ID", "FOLDER", "GOAL", "SEQUENCE", "CREATED"}}
	for _, fg := range g.Items {
		created := ""
		if fg.CreatedDatetime != nil {
			created = fg.CreatedDatetime.Format("2006-01-02 15:04")
		}
		t.Rows = append(t.Rows, []string{
			ptrStr(fg.Id), fg.FolderId, fg.GoalId, ptrInt(fg.SequenceNo), created,
		})
	}
	return t
}
