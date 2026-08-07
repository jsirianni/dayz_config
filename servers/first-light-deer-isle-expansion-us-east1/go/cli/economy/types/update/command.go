package update

import (
	"fmt"
	"io"
	"strings"

	"dzcli/internal/economy"

	"github.com/spf13/cobra"
)

type collectionFlagSet struct {
	Field  string
	Plural string
}

var scalarFields = []string{"nominal", "lifetime", "restock", "min", "quantmin", "quantmax", "cost"}

var collectionFields = []collectionFlagSet{
	{Field: "category", Plural: "categories"},
	{Field: "tag", Plural: "tags"},
	{Field: "usage", Plural: "usages"},
	{Field: "value", Plural: "values"},
}

var writeFileMutation = economy.WriteFileMutation

func NewCommand(stdout io.Writer) *cobra.Command {
	var dryRun bool
	var occurrence int
	var rename string
	scalarValues := map[string]*int{}

	command := &cobra.Command{
		Use:   "update <types.xml> <type-name>",
		Short: "Modify a type entry in a types XML file",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			options, err := updateOptionsFromCommand(cmd, args[1], occurrence, rename, scalarValues)
			if err != nil {
				return err
			}
			mutation, err := economy.UpdateTypesFile(args[0], options)
			if err != nil {
				return err
			}
			if dryRun {
				_, err := stdout.Write(mutation.Data)
				return err
			}
			if err := writeFileMutation(args[0], mutation); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "types %s ok\n", args[0])
			return nil
		},
	}

	flags := command.Flags()
	flags.BoolVar(&dryRun, "dry-run", false, "print modified XML without writing")
	flags.IntVar(&occurrence, "occurrence", 0, "select a duplicate type occurrence")
	flags.StringVar(&rename, "rename", "", "rename the type")
	flags.StringArray("flag", nil, "set a flags attribute as name=0|1")
	flags.StringArray("remove-flag", nil, "remove a flags attribute by name")
	flags.Bool("remove-flags", false, "remove the flags field")
	for _, field := range scalarFields {
		value := 0
		scalarValues[field] = &value
		flags.IntVar(&value, field, 0, "set "+field)
	}
	for _, collection := range collectionFields {
		flags.StringArray("add-"+collection.Field, nil, "add a "+collection.Field+" reference")
		flags.StringArray("remove-"+collection.Field, nil, "remove a "+collection.Field+" reference")
		flags.StringArray("set-"+collection.Field, nil, "replace "+collection.Field+" references")
		flags.Bool("clear-"+collection.Plural, false, "remove all "+collection.Field+" references")
	}
	return command
}

func updateOptionsFromCommand(cmd *cobra.Command, typeName string, occurrence int, rename string, scalarValues map[string]*int) (economy.TypeUpdateOptions, error) {
	options := economy.TypeUpdateOptions{
		TypeName:    typeName,
		Scalars:     map[string]int{},
		Flags:       map[string]string{},
		Collections: map[string]economy.CollectionUpdate{},
	}
	flags := cmd.Flags()
	if flags.Changed("occurrence") {
		options.OccurrenceSet = true
		options.Occurrence = occurrence
	}
	if flags.Changed("rename") {
		options.Rename = rename
	}
	for field, value := range scalarValues {
		if flags.Changed(field) {
			options.Scalars[field] = *value
		}
	}
	flagExpressions, _ := flags.GetStringArray("flag")
	for _, expression := range flagExpressions {
		name, value, err := splitFlagExpression(expression)
		if err != nil {
			return economy.TypeUpdateOptions{}, err
		}
		options.Flags[name] = value
	}
	options.RemoveFlags, _ = flags.GetStringArray("remove-flag")
	options.RemoveAllFlags, _ = flags.GetBool("remove-flags")
	for _, collection := range collectionFields {
		update := collectionUpdateFromFlags(flags, collection)
		if update.Set != nil || update.Clear || len(update.Add) > 0 || len(update.Remove) > 0 {
			options.Collections[collection.Field] = update
		}
	}
	return options, nil
}

func splitFlagExpression(expression string) (string, string, error) {
	name, value, ok := strings.Cut(expression, "=")
	if !ok || strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
		return "", "", fmt.Errorf("--flag expected name=0|1, got %q", expression)
	}
	return name, value, nil
}

func collectionUpdateFromFlags(flags interface {
	GetStringArray(string) ([]string, error)
	GetBool(string) (bool, error)
	Changed(string) bool
}, collection collectionFlagSet) economy.CollectionUpdate {
	add, _ := flags.GetStringArray("add-" + collection.Field)
	remove, _ := flags.GetStringArray("remove-" + collection.Field)
	set, _ := flags.GetStringArray("set-" + collection.Field)
	clear, _ := flags.GetBool("clear-" + collection.Plural)
	update := economy.CollectionUpdate{Add: add, Remove: remove, Clear: clear}
	if flags.Changed("set-" + collection.Field) {
		update.Set = set
	}
	return update
}
