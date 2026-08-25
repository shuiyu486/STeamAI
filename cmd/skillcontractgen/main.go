package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/shuiyu486/re-context-kits/internal/rekit/skillcontract"
)

func main() {
	repo := flag.String("repo", ".", "repository root")
	check := flag.Bool("check", false, "validate without writing")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintln(
			os.Stderr,
			"skillcontractgen accepts only -repo and -check",
		)
		os.Exit(2)
	}
	var err error
	if *check {
		err = skillcontract.Check(*repo)
	} else {
		err = skillcontract.Synchronize(*repo)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
