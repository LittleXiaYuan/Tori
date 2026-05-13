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
			var value any = cognisdk.JSONSchemaInfos()
			if listOptions.WithSchema {
				value = schemaCatalogEntriesWithSchema()
			}
			data, err := json.MarshalIndent(value, "", "  ")
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
	if args[0] == "export" {
		exportOptions, err := parseExportOptions(args[1:])
		if err != nil {
			return err
		}
		artifacts, err := cognisdk.ExportJSONSchemaArtifacts(exportOptions.Dir)
		if err != nil {
			return err
		}
		if exportOptions.Catalog != "" {
			data, err := json.MarshalIndent(artifacts, "", "  ")
			if err != nil {
				return err
			}
			return writeTextFile(exportOptions.Catalog, string(data)+"\n")
		}
		for _, artifact := range artifacts {
			fmt.Println(artifact.File)
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
	fmt.Println("  cognisdk-schema list [--json] [--with-schema] [--out schema-catalog.json]")
	fmt.Println("  cognisdk-schema export <output-dir> [--catalog schema-artifacts.json]")
	fmt.Println("  cognisdk-schema <schema-name> [output.json]")
	fmt.Println("")
	fmt.Println("Schema names:")
	for _, name := range cognisdk.JSONSchemaNames() {
		fmt.Printf("  - %s\n", name)
	}
}

type listOptions struct {
	JSON       bool
	WithSchema bool
	Out        string
}

func parseListOptions(args []string) (listOptions, error) {
	var opts listOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			opts.JSON = true
		case "--with-schema":
			opts.WithSchema = true
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
	if opts.WithSchema && !opts.JSON {
		return listOptions{}, fmt.Errorf("--with-schema requires --json")
	}
	return opts, nil
}

type exportOptions struct {
	Dir     string
	Catalog string
}

func parseExportOptions(args []string) (exportOptions, error) {
	var opts exportOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--catalog":
			if i+1 >= len(args) {
				return exportOptions{}, fmt.Errorf("--catalog requires a path")
			}
			opts.Catalog = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return exportOptions{}, fmt.Errorf("unknown export option %q", args[i])
			}
			if opts.Dir != "" {
				return exportOptions{}, fmt.Errorf("usage: cognisdk-schema export <output-dir> [--catalog schema-artifacts.json]")
			}
			opts.Dir = args[i]
		}
	}
	if opts.Dir == "" {
		return exportOptions{}, fmt.Errorf("usage: cognisdk-schema export <output-dir> [--catalog schema-artifacts.json]")
	}
	return opts, nil
}

type schemaCatalogEntry struct {
	cognisdk.JSONSchemaInfo
	SchemaDocument cognisdk.JSONSchema `json:"schema_document" yaml:"schema_document"`
}

func schemaCatalogEntriesWithSchema() []schemaCatalogEntry {
	infos := cognisdk.JSONSchemaInfos()
	entries := make([]schemaCatalogEntry, 0, len(infos))
	for _, info := range infos {
		schema, ok := cognisdk.JSONSchemaByName(info.Name)
		if !ok {
			continue
		}
		entries = append(entries, schemaCatalogEntry{
			JSONSchemaInfo: info,
			SchemaDocument: schema,
		})
	}
	return entries
}

func writeTextFile(path, value string) error {
	if err := os.WriteFile(path, []byte(value), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}
