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

	initvalidate "dzcli/cli/dayzinit/validate"
	"dzcli/cli/economy/limits"
	typeupdate "dzcli/cli/economy/types/update"
	cevalidate "dzcli/cli/economy/validate"
	"dzcli/cli/expansion/ai/loadouts"
	"dzcli/cli/expansion/ai/patrols"
	aivalidate "dzcli/cli/expansion/ai/validate"
	serverconfigcmd "dzcli/cli/server/config"
	servergameplaycmd "dzcli/cli/server/gameplay"
	serverweathercmd "dzcli/cli/server/weather"
	xmlvalidate "dzcli/cli/xml/validate"
	"dzcli/internal/economy"
	"dzcli/internal/economyconfig"
	"dzcli/internal/expansion"

	"github.com/spf13/cobra"
)

var getwd = os.Getwd

func NewGetCommand(stdout io.Writer) *cobra.Command {
	command := newParent("get", "List DayZ configuration resources")
	command.SetOut(stdout)
	command.AddCommand(newGetEconomyCommand(stdout))
	command.AddCommand(newGetExpansionCommand(stdout))
	command.AddCommand(serverconfigcmd.NewGetCommand(stdout))
	command.AddCommand(servergameplaycmd.NewGetCommand(stdout))
	return command
}

func NewCreateCommand(stdout io.Writer) *cobra.Command {
	command := newParent("create", "Create DayZ configuration resources")
	command.SetOut(stdout)
	command.AddCommand(newEconomyParent(stdout, limits.NewCreateCommand(stdout)))
	command.AddCommand(newExpansionParent(stdout, newAIParent(stdout, patrols.NewCreateCommand(stdout), loadouts.NewCreateCommand(stdout))))
	return command
}

func NewUpdateCommand(stdin io.Reader, stdout io.Writer) *cobra.Command {
	command := newParent("update", "Update DayZ configuration resources")
	command.SetOut(stdout)
	command.AddCommand(newEconomyParent(stdout, typeupdate.NewCommand(stdout)))
	command.AddCommand(newExpansionParent(stdout, newAIParent(stdout, patrols.NewUpdateCommand(stdout), loadouts.NewUpdateCommand(stdout))))
	command.AddCommand(serverconfigcmd.NewUpdateCommand(stdin, stdout))
	command.AddCommand(servergameplaycmd.NewUpdateCommand(stdin, stdout))
	return command
}

func NewDeleteCommand(stdin io.Reader, stdout io.Writer) *cobra.Command {
	command := newParent("delete", "Delete DayZ configuration resources")
	command.SetOut(stdout)
	command.AddCommand(newEconomyParent(stdout, limits.NewDeleteCommand(stdout)))
	command.AddCommand(newExpansionParent(stdout, newAIParent(stdout, patrols.NewDeleteCommand(stdout), loadouts.NewDeleteCommandWithInput(stdin, stdout))))
	command.AddCommand(serverconfigcmd.NewDeleteCommand(stdout))
	command.AddCommand(servergameplaycmd.NewDeleteCommand(stdout))
	return command
}

func NewValidateCommand(stdout io.Writer) *cobra.Command {
	command := newParent("validate", "Validate DayZ configuration files")
	command.SetOut(stdout)
	command.AddCommand(initvalidate.NewCommand(stdout))
	command.AddCommand(serverconfigcmd.NewValidateCommand(stdout))
	command.AddCommand(servergameplaycmd.NewValidateCommand(stdout))
	command.AddCommand(serverweathercmd.NewValidateCommand(stdout))
	command.AddCommand(cevalidate.NewCommand(stdout))
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
	for _, path := range files {
		types, err := economyconfig.ParseTypesFile(path)
		if err != nil {
			return err
		}
		for _, entry := range types.Types {
			if filter != "" && entry.Name != filter {
				continue
			}
			rows = append(rows, []string{entry.Name, displayFileName(path)})
		}
	}
	sortRows(rows, 0, 1)
	return printMatchedRows(stdout, []string{"NAME", "FILE"}, rows, "type", filter)
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
	core, err := economyconfig.ParseEconomyCoreFile(economyCore)
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
			rows = append(rows, []string{name})
		}
	}
	return printMatchedRows(stdout, []string{"NAME"}, rows, kind, filter)
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
			rows = append(rows, []string{group.Name, strings.Join(group.Members, ",")})
		}
	}
	return printMatchedRows(stdout, []string{"NAME", "MEMBERS"}, rows, kind+" group", filter)
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
	for index, patrol := range settings.Patrols {
		if filter != "" && patrol.Name != filter {
			continue
		}
		rows = append(rows, []string{
			patrol.Name,
			strconv.Itoa(index + 1),
			patrol.Loadout,
			patrol.Faction,
			patrol.Behaviour,
			formatPatrolCoordinates(patrol),
		})
	}
	sortRows(rows, 0, 1)
	return printMatchedRows(stdout, []string{"NAME", "INDEX", "LOADOUT", "FACTION", "BEHAVIOUR", "COORDINATES"}, rows, "patrol", filter)
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
		rows = append(rows, []string{name, strconv.Itoa(expansion.CountPrefabItems(loadout))})
	}
	sortRows(rows, 0)
	return printMatchedRows(stdout, []string{"NAME", "ITEMS"}, rows, "loadout", filter)
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

func formatPatrolCoordinates(patrol expansion.AIPatrol) string {
	if len(patrol.Waypoints) == 0 {
		return ""
	}
	waypoint := patrol.Waypoints[0]
	return strings.Join([]string{
		formatCoordinate(waypoint[0]),
		formatCoordinate(waypoint[1]),
		formatCoordinate(waypoint[2]),
	}, ",")
}

func formatCoordinate(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func displayFileName(path string) string {
	return filepath.Base(filepath.Clean(path))
}

func printMatchedRows(stdout io.Writer, headers []string, rows [][]string, resource string, filter string) error {
	if len(rows) == 0 {
		if filter != "" {
			return fmt.Errorf("%s %q not found", resource, filter)
		}
		return fmt.Errorf("no %s resources found", resource)
	}
	headers, rows = dropEmptyNameColumn(headers, rows)
	sortNamelessIndexRows(headers, rows)
	printTable(stdout, headers, rows)
	return nil
}

func dropEmptyNameColumn(headers []string, rows [][]string) ([]string, [][]string) {
	nameColumn := headerIndex(headers, "NAME")
	if nameColumn == -1 {
		return headers, rows
	}
	for _, row := range rows {
		if nameColumn < len(row) && strings.TrimSpace(row[nameColumn]) != "" {
			return headers, rows
		}
	}

	trimmedHeaders := removeColumn(headers, nameColumn)
	trimmedRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		trimmedRows = append(trimmedRows, removeColumn(row, nameColumn))
	}
	return trimmedHeaders, trimmedRows
}

func sortNamelessIndexRows(headers []string, rows [][]string) {
	if headerIndex(headers, "NAME") != -1 {
		return
	}
	indexColumn := headerIndex(headers, "INDEX")
	if indexColumn == -1 {
		return
	}
	sort.SliceStable(rows, func(left, right int) bool {
		leftIndex, _ := strconv.Atoi(rows[left][indexColumn])
		rightIndex, _ := strconv.Atoi(rows[right][indexColumn])
		return leftIndex < rightIndex
	})
}

func headerIndex(headers []string, name string) int {
	for index, header := range headers {
		if header == name {
			return index
		}
	}
	return -1
}

func removeColumn(values []string, index int) []string {
	trimmed := make([]string, 0, len(values)-1)
	trimmed = append(trimmed, values[:index]...)
	trimmed = append(trimmed, values[index+1:]...)
	return trimmed
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
