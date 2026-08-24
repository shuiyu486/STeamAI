package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shuiyu486/re-context-kits/internal/rekit/skillcontract"
)

func main() {
	repo := flag.String("repo", ".", "repository root")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "skillcontractgen accepts only -repo")
		os.Exit(2)
	}
	if err := skillcontract.Synchronize(*repo); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
