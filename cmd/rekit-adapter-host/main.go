package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/shuiyu486/re-context-kits/internal/rekit/adapterhost"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if adapterhost.IsEmbeddedPrivateInvocation(args) {
		for _, arg := range args {
			name, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(arg)), "=")
			if name == "-prepare-vmp-ida-index-request" {
				fmt.Fprintln(os.Stderr, "request prepare, authorized parent run, and private child modes are mutually exclusive")
				return 2
			}
		}
		if handled, code := adapterhost.RunEmbeddedPrivate(args, os.Stdout, os.Stderr); handled {
			return code
		}
	}
	flags := flag.NewFlagSet("rekit-adapter-host", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: rekit-adapter-host -repo <kit> -target <case> -pack <pack> -gate-event-id <id> -expected-dispatch-sha256 <sha256>")
	}
	var opt adapterhost.Options
	var prepare, apply bool
	var exportRoot, expectedRequestSHA, queryTerms string
	var maxRowsPerIndex int
	flags.StringVar(&opt.RepoRoot, "repo", "", "canonical re-context-kits repository root")
	flags.StringVar(&opt.CaseRoot, "target", "", "attached case root")
	flags.StringVar(&opt.Pack, "pack", "", "attached pack")
	flags.StringVar(&opt.GateEventID, "gate-event-id", "", "authorized gate event id")
	flags.StringVar(&opt.ExpectedDispatchSHA256, "expected-dispatch-sha256", "", "immutable pre-execution dispatch sha256")
	flags.BoolVar(&prepare, "prepare-vmp-ida-index-request", false, "")
	flags.StringVar(&exportRoot, "export-root", adapterhost.VMPIDAIndexDefaultExportRoot, "case-relative fixed IDA export root")
	flags.StringVar(&queryTerms, "terms", "", "comma-separated literal query terms")
	flags.IntVar(&maxRowsPerIndex, "max-rows-per-index", 50, "maximum selected rows per fixed index")
	flags.BoolVar(&apply, "apply", false, "publish the exact prepared request")
	flags.StringVar(&expectedRequestSHA, "expected-request-sha256", "", "expected prepared request sha256")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "rekit-adapter-host does not accept positional arguments")
		return 2
	}
	if !prepare && (apply || strings.TrimSpace(queryTerms) != "" || maxRowsPerIndex != 50) {
		fmt.Fprintln(os.Stderr, "-apply, -terms, and -max-rows-per-index are supported only with -prepare-vmp-ida-index-request")
		return 2
	}
	var result any
	var err error
	if prepare {
		var preview adapterhost.VMPIDAIndexRequestPreview
		var previewErr error
		if strings.TrimSpace(queryTerms) == "" {
			preview, previewErr = adapterhost.PreviewVMPIDAIndexRequest(opt.CaseRoot, exportRoot)
		} else {
			terms := strings.Split(queryTerms, ",")
			for index := range terms {
				terms[index] = strings.TrimSpace(terms[index])
			}
			preview, previewErr = adapterhost.PreviewVMPIDAIndexRequestForQuery(opt.CaseRoot, exportRoot, adapterhost.VMPIDAIndexQuery{
				SchemaVersion: 1, Terms: terms, MaxRowsPerIndex: maxRowsPerIndex,
			})
		}
		if previewErr != nil {
			err = previewErr
		} else if !apply {
			result = preview
		} else if !strings.EqualFold(strings.TrimSpace(expectedRequestSHA), preview.RequestSHA256) {
			err = fmt.Errorf("prepared request sha256 mismatch: expected %s got %s", expectedRequestSHA, preview.RequestSHA256)
		} else {
			result, err = adapterhost.PublishVMPIDAIndexRequest(opt.CaseRoot, preview)
		}
	} else {
		result, err = adapterhost.Run(opt)
	}
	if result != nil {
		data, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr == nil {
			fmt.Println(string(data))
		} else if err == nil {
			err = marshalErr
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
