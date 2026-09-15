// Command yaml-get extracts a single scalar value from a YAML file by a
// dot-separated path, for use in shell scripts and CI without pulling in
// a full YAML toolchain.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	file := flag.String("f", "", "path to YAML file (default: stdin)")
	var defaultVal string
	flag.StringVar(&defaultVal, "d", "", "value to print if the path is not found, instead of exiting non-zero")
	flag.StringVar(&defaultVal, "default", "", "same as -d")
	setVal := flag.String("set", "", "write this value at <path> instead of printing it (requires -f; overwrites the file)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: yaml-get [-f file] [-d default] <path>")
		fmt.Fprintln(os.Stderr, "       yaml-get [-f file] -set value <path>")
		flag.PrintDefaults()
	}
	flag.Parse()

	haveDefault := false
	haveSet := false
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "d", "default":
			haveDefault = true
		case "set":
			haveSet = true
		}
	})

	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(1)
	}
	path := args[0]

	segs, err := ParsePath(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "yaml-get: %v\n", err)
		os.Exit(1)
	}

	if haveSet && (*file == "" || *file == "-") {
		fmt.Fprintln(os.Stderr, "yaml-get: -set requires -f <file>")
		os.Exit(1)
	}

	var src []byte
	if *file == "" || *file == "-" {
		src, err = io.ReadAll(os.Stdin)
	} else {
		src, err = os.ReadFile(*file)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "yaml-get: %v\n", err)
		os.Exit(1)
	}

	if haveSet {
		newSrc, err := Set(string(src), segs, *setVal)
		if err != nil {
			fmt.Fprintf(os.Stderr, "yaml-get: %v\n", err)
			os.Exit(2)
		}
		perm := os.FileMode(0644)
		if info, statErr := os.Stat(*file); statErr == nil {
			perm = info.Mode().Perm()
		}
		if err := os.WriteFile(*file, []byte(newSrc), perm); err != nil {
			fmt.Fprintf(os.Stderr, "yaml-get: %v\n", err)
			os.Exit(1)
		}
		return
	}

	root, err := Parse(string(src))
	if err != nil {
		fmt.Fprintf(os.Stderr, "yaml-get: %v\n", err)
		os.Exit(1)
	}

	val, err := Lookup(root, segs)
	if err != nil {
		var notFound *NotFoundError
		if haveDefault && errors.As(err, &notFound) {
			fmt.Println(defaultVal)
			return
		}
		fmt.Fprintf(os.Stderr, "yaml-get: %v\n", err)
		os.Exit(2)
	}

	switch v := val.(type) {
	case map[string]Node, []Node:
		fmt.Fprintf(os.Stderr, "yaml-get: value at %q is not a scalar\n", path)
		os.Exit(2)
	case nil:
		fmt.Println()
	default:
		fmt.Println(v)
	}
}
