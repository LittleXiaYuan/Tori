package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"yunque-agent/pkg/cognisdk"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
	}
	if args[0] == "list" {
		listOptions, err := parseListOptions(args[1:])
		if err != nil {
			return err
		}
		if listOptions.JSON {
			data, err := json.MarshalIndent(cognisdk.JSONSchemaInfos(), "", "  ")
			if err != nil {
				return err
			}
			if listOptions.Out != "" {
				return writeTextFile(listOptions.Out, string(data)+"\n")
			}
			fmt.Println(string(data))
			return nil
		}
		text := strings.Join(cognisdk.JSONSchemaNames(), "\n") + "\n"
		if listOptions.Out != "" {
			return writeTextFile(listOptions.Out, text)
		}
		fmt.Print(text)
		return nil
	}

	schema, ok := cognisdk.JSONSchemaByName(args[0])
	if !ok {
		return fmt.Errorf("unknown schema %q; available: %s", args[0], strings.Join(cognisdk.JSONSchemaNames(), ", "))
	}
	if len(args) > 2 {
		return fmt.Errorf("too many arguments")
	}
	if len(args) == 2 {
		return cognisdk.SaveJSONSchema(schema, args[1])
	}
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  cognisdk-schema list [--json] [--out schema-catalog.json]")
	fmt.Println("  cognisdk-schema <schema-name> [output.json]")
	fmt.Println("")
	fmt.Println("Schema names:")
	for _, name := range cognisdk.JSONSchemaNames() {
		fmt.Printf("  - %s\n", name)
	}
}

type listOptions struct {
	JSON bool
	Out  string
}

func parseListOptions(args []string) (listOptions, error) {
	var opts listOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			opts.JSON = true
		case "--out":
			if i+1 >= len(args) {
				return listOptions{}, fmt.Errorf("--out requires a path")
			}
			opts.Out = args[i+1]
			i++
		default:
			return listOptions{}, fmt.Errorf("unknown list option %q", args[i])
		}
	}
	return opts, nil
}

func writeTextFile(path, value string) error {
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}
