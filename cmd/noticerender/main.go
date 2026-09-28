package main

import (
	"encoding/json"
	"fmt"
	"os"

	"example.com/localized-notification-renderer/internal/catalog"
	tplpkg "example.com/localized-notification-renderer/internal/pkg"
	"example.com/localized-notification-renderer/internal/render"
)

const usage = `noticerender - localized notification template tool

Usage:
  noticerender compile -catalog catalog.json -out package.json
  noticerender render  -package package.json -requests requests.jsonl \
                       [-results results.jsonl] [-summary summary.json]

The whole catalog is validated before a package is written; on any validation
error the output package is not created or overwritten.

Render reads JSONL requests and writes one JSON result per request, in input
order, then a summary JSON with success/failure/fallback counts. Results and
summary default to stdout and stderr when the corresponding flags are omitted.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "compile":
		err = runCompile(os.Args[2:])
	case "render":
		err = runRender(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		err = fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "noticerender: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags(args []string) (map[string]string, error) {
	flags := make(map[string]string)
	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) < 2 || a[0] != '-' {
			return nil, fmt.Errorf("unexpected argument %q", a)
		}
		key := a[1:]
		if len(key) > 0 && key[0] == '-' {
			key = key[1:]
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("flag %q requires a value", a)
		}
		flags[key] = args[i+1]
		i++
	}
	return flags, nil
}

func runCompile(args []string) error {
	flags, err := parseFlags(args)
	if err != nil {
		return err
	}
	catalogPath, ok := flags["catalog"]
	if !ok {
		return fmt.Errorf("compile requires -catalog")
	}
	outPath, ok := flags["out"]
	if !ok {
		return fmt.Errorf("compile requires -out")
	}

	data, err := os.ReadFile(catalogPath)
	if err != nil {
		return err
	}
	var c catalog.Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("decode catalog: %w", err)
	}
	compiled, err := catalog.Compile(&c)
	if err != nil {
		return err
	}
	// Package is written only after full validation, atomically.
	if err := tplpkg.FromCompiled(compiled).WriteFile(outPath); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "compiled %d template(s) into %s\n", len(compiled.TemplateIDs), outPath)
	return nil
}

func runRender(args []string) error {
	flags, err := parseFlags(args)
	if err != nil {
		return err
	}
	packagePath, ok := flags["package"]
	if !ok {
		return fmt.Errorf("render requires -package")
	}
	requestsPath, ok := flags["requests"]
	if !ok {
		return fmt.Errorf("render requires -requests")
	}

	p, err := tplpkg.ReadFile(packagePath)
	if err != nil {
		return err
	}

	reqFile, err := os.Open(requestsPath)
	if err != nil {
		return err
	}
	defer reqFile.Close()

	var resultsFile *os.File = os.Stdout
	if path := flags["results"]; path != "" {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		resultsFile = f
	}

	renderer := render.New(p)
	summary, err := renderer.Stream(reqFile, resultsFile)
	if err != nil {
		return err
	}

	summaryData, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	summaryData = append(summaryData, '\n')

	var summaryFile *os.File = os.Stderr
	if path := flags["summary"]; path != "" {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		summaryFile = f
	}
	_, err = summaryFile.Write(summaryData)
	return err
}
