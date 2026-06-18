package main

import (
	"fmt"
	"path/filepath"
	"time"
)

func reviewRootPath(opts Options, p Paths, command string, name string) (string, error) {
	if opts.ReviewOutputDir != "" {
		var root string
		if filepath.IsAbs(opts.ReviewOutputDir) {
			root = filepath.Clean(opts.ReviewOutputDir)
		} else {
			root = filepath.Join(p.CaseRoot, filepath.FromSlash(opts.ReviewOutputDir))
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		if !isChildPath(p.CaseRoot, abs) {
			return "", fmt.Errorf("review output dir must stay inside case root: %s", root)
		}
		return abs, nil
	}
	m := loadPackManifestForPaths(p)
	reviewRoot, err := joinInside(p.CaseRoot, m.ParallelDefaults.ReviewRoot)
	if err != nil {
		return "", fmt.Errorf("invalid parallelDefaults.reviewRoot: %w", err)
	}
	return filepath.Join(reviewRoot, time.Now().Format("20060102-150405000")+"-"+command+"-"+name), nil
}

func packetPath(opts Options, reviewRoot string) (string, error) {
	if opts.PacketPath != "" {
		var path string
		if filepath.IsAbs(opts.PacketPath) {
			path = filepath.Clean(opts.PacketPath)
		} else {
			path = filepath.Join(reviewRoot, filepath.FromSlash(opts.PacketPath))
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		if !isChildPath(reviewRoot, abs) {
			return "", fmt.Errorf("packet path must stay inside review root: %s", path)
		}
		if samePath(abs, summaryPath(reviewRoot)) {
			return "", fmt.Errorf("packet path conflicts with summary output: %s", path)
		}
		return abs, nil
	}
	return filepath.Join(reviewRoot, "packet.json"), nil
}

func summaryPath(reviewRoot string) string {
	return filepath.Join(reviewRoot, "summary.md")
}
