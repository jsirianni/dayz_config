package verbs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"dzcli/cli/economy/limits"
	typeupdate "dzcli/cli/economy/types/update"
	cevalidate "dzcli/cli/economy/validate"
	"dzcli/cli/expansion/ai/loadouts"
	"dzcli/cli/expansion/ai/patrols"
	aivalidate "dzcli/cli/expansion/ai/validate"
	xmlvalidate "dzcli/cli/xml/validate"
	"dzcli/internal/economy"
	"dzcli/internal/expansion"

	"github.com/spf13/cobra"
)

var getwd = os.Getwd

func NewGetCommand(stdout io.Writer) *cobra.Command {
	command := newParent("get", "List DayZ configuration resources")
	command.SetOut(stdout)
	command.AddCommand(newGetEconomyCommand(stdout))
	command.AddCommand(newGetExpansionCommand(stdout))
	return command
}

func NewCreateCommand(stdout io.Writer) *cobra.Command {
	command := newParent("create", "Create DayZ configuration resources")
	command.SetOut(stdout)
	command.AddCommand(newEconomyParent(stdout, limits.NewCreateCommand(stdout)))
	command.AddCommand(newExpansionParent(stdout, newAIParent(stdout, patrols.NewCreateCommand(stdout), loadouts.NewCreateCommand(stdout))))
	return command
}

func NewUpdateCommand(stdout io.Writer) *cobra.Command {
	command := newParent("update", "Update DayZ configuration resources")
	command.SetOut(stdout)
	command.AddCommand(newEconomyParent(stdout, typeupdate.NewCommand(stdout)))
	command.AddCommand(newExpansionParent(stdout, newAIParent(stdout, patrols.NewUpdateCommand(stdout), loadouts.NewUpdateCommand(stdout))))
	return command
}

func NewDeleteCommand(stdin io.Reader, stdout io.Writer) *cobra.Command {
	command := newParent("delete", "Delete DayZ configuration resources")
	command.SetOut(stdout)
	command.AddCommand(newEconomyParent(stdout, limits.NewDeleteCommand(stdout)))
	command.AddCommand(newExpansionParent(stdout, newAIParent(stdout, patrols.NewDeleteCommand(stdout), loadouts.NewDeleteCommandWithInput(stdin, stdout))))
	return command
}

func NewValidateCommand(stdout io.Writer) *cobra.Command {
	command := newParent("validate", "Validate DayZ configuration files")
	command.SetOut(stdout)
	command.AddCommand(&cobra.Command{
		Use:   "economy <cfgeconomycore.xml>",
		Short: "Validate cfgeconomycore.xml and referenced types files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cevalidate.ValidateEconomyCore(args[0], stdout)
		},
	})
	expansionCommand := newParent("expansion", "Validate DayZ Expansion mod configuration")
	expansionCommand.AddCommand(&cobra.Command{
		Use:   "ai [path]",
		Short: "Validate DayZ Expansion AI patrol and loadout files",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			return aivalidate.ValidateAIPath(path, stdout)
		},
	})
	command.AddCommand(expansionCommand)
	command.AddCommand(&cobra.Command{
		Use:   "xml [path]",
		Short: "Validate XML files recursively",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "."
			if len(args) == 1 {
				path = args[0]
			}
			return xmlvalidate.ValidateXMLPath(path, stdout)
		},
	})
	return command
}

func newGetEconomyCommand(stdout io.Writer) *cobra.Command {
	command := newParent("economy", "List DayZ central economy resources")
	command.AddCommand(newGetEconomyTypesCommand(stdout))
	command.AddCommand(newGetEconomyLimitsCommand(stdout))
	return command
}

func newGetEconomyTypesCommand(stdout io.Writer) *cobra.Command {
	var file string
	var economyCore string
	command := &cobra.Command{
		Use:   "types [name]",
		Short: "List type entries",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}
			return listEconomyTypes(file, economyCore, filter, stdout)
		},
	}
	command.Flags().StringVar(&file, "file", "", "types.xml path")
	command.Flags().StringVar(&economyCore, "cfgeconomycore", "", "cfgeconomycore.xml path")
	return command
}

func newGetEconomyLimitsCommand(stdout io.Writer) *cobra.Command {
	var file string
	command := &cobra.Command{
		Use:   "limits <category|tag|usage|value> [name]",
		Short: "List base limits definitions",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := ""
			if len(args) == 2 {
				filter = args[1]
			}
			return listEconomyLimits(file, args[0], filter, stdout)
		},
	}
	command.Flags().StringVar(&file, "file", "", "cfglimitsdefinition.xml path")
	command.AddCommand(newGetEconomyLimitGroupsCommand(stdout))
	return command
}

func newGetEconomyLimitGroupsCommand(stdout io.Writer) *cobra.Command {
	var file string
	command := &cobra.Command{
		Use:   "group <usage|value> [name]",
		Short: "List user limits groups",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := ""
			if len(args) == 2 {
				filter = args[1]
			}
			return listEconomyLimitGroups(file, args[0], filter, stdout)
		},
	}
	command.Flags().StringVar(&file, "file", "", "cfglimitsdefinitionuser.xml path")
	return command
}

func newGetExpansionCommand(stdout io.Writer) *cobra.Command {
	command := newParent("expansion", "List DayZ Expansion mod resources")
	command.AddCommand(newAIParent(stdout, newGetExpansionAIPatrolsCommand(stdout), newGetExpansionAILoadoutsCommand(stdout)))
	return command
}

func newGetExpansionAIPatrolsCommand(stdout io.Writer) *cobra.Command {
	var file string
	command := &cobra.Command{
		Use:   "patrols [name]",
		Short: "List DayZ Expansion AI patrols",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}
			return listExpansionAIPatrols(file, filter, stdout)
		},
	}
	command.Flags().StringVar(&file, "file", "", "AIPatrolSettings.json path")
	return command
}

func newGetExpansionAILoadoutsCommand(stdout io.Writer) *cobra.Command {
	var file string
	var path string
	command := &cobra.Command{
		Use:   "loadouts [name]",
		Short: "List DayZ Expansion AI loadouts",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := ""
			if len(args) == 1 {
				filter = expansion.NormalizeLoadoutName(args[0])
			}
			return listExpansionAILoadouts(file, path, filter, stdout)
		},
	}
	command.Flags().StringVar(&file, "file", "", "loadout JSON path")
	command.Flags().StringVar(&path, "path", "", "repo or server root path")
	return command
}

func listEconomyTypes(file string, economyCore string, filter string, stdout io.Writer) error {
	files, err := economyTypeFiles(file, economyCore)
	if err != nil {
		return err
	}
	var rows [][]string
	occurrences := map[string]int{}
	for _, path := range files {
		types, err := cevalidate.ParseTypesFile(path)
		if err != nil {
			return err
		}
		for _, entry := range types.Types {
			if filter != "" && entry.Name != filter {
				continue
			}
			occurrences[entry.Name]++
			rows = append(rows, []string{"type", entry.Name, path, strconv.Itoa(occurrences[entry.Name])})
		}
	}
	sortRows(rows, 1, 2, 3)
	return printMatchedRows(stdout, []string{"KIND", "NAME", "FILE", "OCCURRENCE"}, rows, "type", filter)
}

func economyTypeFiles(file string, economyCore string) ([]string, error) {
	if file != "" && economyCore != "" {
		return nil, fmt.Errorf("use --file or --cfgeconomycore, not both")
	}
	if file != "" {
		return []string{file}, nil
	}
	if economyCore == "" {
		return nil, fmt.Errorf("--file or --cfgeconomycore is required")
	}
	core, err := cevalidate.ParseEconomyCoreFile(economyCore)
	if err != nil {
		return nil, err
	}
	missionDir := filepath.Dir(filepath.Clean(economyCore))
	files := []string{filepath.Join(missionDir, "db", "types.xml")}
	for _, ref := range core.TypeFileRefs() {
		files = append(files, filepath.Join(missionDir, ref.Folder, ref.Name))
	}
	return files, nil
}

func listEconomyLimits(file string, kind string, filter string, stdout io.Writer) error {
	if file == "" {
		return fmt.Errorf("--file is required")
	}
	names, err := economy.ListLimitNamesFile(file, kind)
	if err != nil {
		return err
	}
	var rows [][]string
	for _, name := range names {
		if filter == "" || name == filter {
			rows = append(rows, []string{kind, name, file})
		}
	}
	return printMatchedRows(stdout, []string{"KIND", "NAME", "FILE"}, rows, kind, filter)
}

func listEconomyLimitGroups(file string, kind string, filter string, stdout io.Writer) error {
	if file == "" {
		return fmt.Errorf("--file is required")
	}
	groups, err := economy.ListUserLimitGroupsFile(file, kind)
	if err != nil {
		return err
	}
	var rows [][]string
	for _, group := range groups {
		if filter == "" || group.Name == filter {
			rows = append(rows, []string{kind + "-group", group.Name, file, strings.Join(group.Members, ",")})
		}
	}
	return printMatchedRows(stdout, []string{"KIND", "NAME", "FILE", "MEMBERS"}, rows, kind+" group", filter)
}

func listExpansionAIPatrols(file string, filter string, stdout io.Writer) error {
	targetFile, err := resolvePatrolsFile(file)
	if err != nil {
		return err
	}
	settings, err := expansion.ParseAIPatrolSettingsFile(targetFile)
	if err != nil {
		return err
	}
	var rows [][]string
	occurrences := map[string]int{}
	for index, patrol := range settings.Patrols {
		occurrences[patrol.Name]++
		if filter != "" && patrol.Name != filter {
			continue
		}
		rows = append(rows, []string{
			"patrol",
			patrol.Name,
			targetFile,
			strconv.Itoa(index + 1),
			strconv.Itoa(occurrences[patrol.Name]),
			patrol.Loadout,
			patrol.Faction,
			patrol.Behaviour,
		})
	}
	sortRows(rows, 1, 3)
	return printMatchedRows(stdout, []string{"KIND", "NAME", "FILE", "INDEX", "OCCURRENCE", "LOADOUT", "FACTION", "BEHAVIOUR"}, rows, "patrol", filter)
}

func listExpansionAILoadouts(file string, path string, filter string, stdout io.Writer) error {
	files, err := loadoutFiles(file, path)
	if err != nil {
		return err
	}
	var rows [][]string
	for _, loadoutFile := range files {
		name := expansion.LoadoutName(loadoutFile)
		if filter != "" && name != filter {
			continue
		}
		loadout, err := expansion.ParseLoadoutFile(loadoutFile)
		if err != nil {
			return err
		}
		rows = append(rows, []string{"loadout", name, loadoutFile, strconv.Itoa(expansion.CountPrefabItems(loadout))})
	}
	sortRows(rows, 1, 2)
	return printMatchedRows(stdout, []string{"KIND", "NAME", "FILE", "ITEMS"}, rows, "loadout", filter)
}

func loadoutFiles(file string, path string) ([]string, error) {
	if file != "" {
		return []string{file}, nil
	}
	if path != "" {
		files, err := expansion.DiscoverAIConfigFiles(path)
		if err != nil {
			return nil, err
		}
		return filterAIFilePaths(files, expansion.KindAILoadout), nil
	}
	wd, err := getwd()
	if err != nil {
		return nil, err
	}
	files, err := expansion.DiscoverAIConfigFilesNear(wd)
	if err != nil {
		return nil, err
	}
	return filterAIFilePaths(files, expansion.KindAILoadout), nil
}

func filterAIFilePaths(files []expansion.AIFile, kind string) []string {
	var paths []string
	for _, file := range files {
		if file.Kind == kind {
			paths = append(paths, file.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

func resolvePatrolsFile(file string) (string, error) {
	if file != "" {
		return file, nil
	}
	wd, err := getwd()
	if err != nil {
		return "", err
	}
	return expansion.FindAIPatrolsFileNear(wd)
}

func printMatchedRows(stdout io.Writer, headers []string, rows [][]string, resource string, filter string) error {
	if len(rows) == 0 {
		if filter != "" {
			return fmt.Errorf("%s %q not found", resource, filter)
		}
		return fmt.Errorf("no %s resources found", resource)
	}
	printTable(stdout, headers, rows)
	return nil
}

func printTable(stdout io.Writer, headers []string, rows [][]string) {
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(writer, strings.Join(row, "\t"))
	}
	_ = writer.Flush()
}

func sortRows(rows [][]string, columns ...int) {
	sort.Slice(rows, func(left, right int) bool {
		for _, column := range columns {
			if rows[left][column] == rows[right][column] {
				continue
			}
			return rows[left][column] < rows[right][column]
		}
		return strings.Join(rows[left], "\x00") < strings.Join(rows[right], "\x00")
	})
}

func newEconomyParent(stdout io.Writer, children ...*cobra.Command) *cobra.Command {
	command := newParent("economy", "Modify DayZ central economy resources")
	command.SetOut(stdout)
	command.AddCommand(children...)
	return command
}

func newExpansionParent(stdout io.Writer, children ...*cobra.Command) *cobra.Command {
	command := newParent("expansion", "Modify DayZ Expansion mod resources")
	command.SetOut(stdout)
	command.AddCommand(children...)
	return command
}

func newAIParent(stdout io.Writer, children ...*cobra.Command) *cobra.Command {
	command := newParent("ai", "Modify DayZ Expansion AI resources")
	command.SetOut(stdout)
	command.AddCommand(children...)
	return command
}

func newParent(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
}
