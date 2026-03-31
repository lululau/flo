package main

import (
	"fmt"
	"os"

	"flo/cmd/flo/cli"
)

func init() {
	cli.RunTUI = RunTUIFunc
}

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
