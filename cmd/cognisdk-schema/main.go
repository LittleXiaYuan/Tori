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
		if len(args) > 2 {
			return fmt.Errorf("usage: cognisdk-schema list [--json]")
		}
		if len(args) == 2 {
			if args[1] != "--json" {
				return fmt.Errorf("unknown list option %q", args[1])
			}
			data, err := json.MarshalIndent(cognisdk.JSONSchemaInfos(), "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(data))
			return nil
		}
		for _, name := range cognisdk.JSONSchemaNames() {
			fmt.Println(name)
		}
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
	fmt.Println("  cognisdk-schema list [--json]")
	fmt.Println("  cognisdk-schema <schema-name> [output.json]")
	fmt.Println("")
	fmt.Println("Schema names:")
	for _, name := range cognisdk.JSONSchemaNames() {
		fmt.Printf("  - %s\n", name)
	}
}
