package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"example.com/localized-notification-renderer/internal/catalog"
	"example.com/localized-notification-renderer/internal/render"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "compile":
		err = runCompile(os.Args[2:])
	case "render":
		err = runRender(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "noticerender:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `noticerender compiles localized notification templates and renders JSONL requests.

Usage:
  noticerender compile <catalog.json> <package.json>
  noticerender render  <package.json> <requests.jsonl> <results.jsonl> <summary.json>

Use "-" for a path to read from stdin (render input only) or write to stdout
(results or summary output). The catalog and package paths are always files.
`)
	os.Exit(2)
}

func runCompile(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("compile needs <catalog.json> <package.json>")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	c, err := catalog.Decode(data)
	if err != nil {
		return err
	}
	pkg, err := catalog.Compile(c)
	if err != nil {
		return fmt.Errorf("catalog validation failed: %w", err)
	}
	out, err := pkg.Marshal()
	if err != nil {
		return err
	}
	if err := writeAtomic(args[1], append(out, '\n')); err != nil {
		return err
	}
	return nil
}

// writeAtomic writes to a temp file in the target directory and renames it
// into place, so a failed compile never leaves a partial/overwritten package
// behind (compile itself validates before calling this).
func writeAtomic(path string, data []byte) error {
	if path == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	dir := filepathDir(path)
	tmp, err := os.CreateTemp(dir, ".noticerender-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func runRender(args []string) error {
	if len(args) != 4 {
		return fmt.Errorf("render needs <package.json> <requests.jsonl> <results.jsonl> <summary.json>")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	pkg, err := catalog.LoadPackage(data)
	if err != nil {
		return err
	}
	in, closeIn, err := openInput(args[1])
	if err != nil {
		return err
	}
	defer closeIn()

	results, summary := render.Batch(pkg, in)

	resultsData, err := encodeJSONL(results)
	if err != nil {
		return err
	}
	summaryData, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	if err := writeAtomic(args[2], resultsData); err != nil {
		return err
	}
	if err := writeAtomic(args[3], append(summaryData, '\n')); err != nil {
		return err
	}
	return nil
}

func openInput(path string) (io.Reader, func(), error) {
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

func encodeJSONL(results []render.Result) ([]byte, error) {
	var buf []byte
	for _, res := range results {
		line, err := json.Marshal(res)
		if err != nil {
			return nil, err
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	return buf, nil
}
