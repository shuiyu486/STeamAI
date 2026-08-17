package sync

import (
	"fmt"
	"sort"
	"strings"
)

type currentSyncStagedFile struct {
	Path string
	Data []byte
	Mode uint32
}

func currentSyncStageMaterial(
	intent currentSyncIntent,
	fresh CurrentSyncPlan,
) ([]currentSyncStagedFile, error) {
	if err := validateCurrentSyncIntentStructure(intent); err != nil {
		return nil, err
	}
	if fresh.prepared == nil || !currentSyncCanonicalEqual(
		currentSyncIdentity(fresh),
		currentSyncIdentity(intent.Plan),
	) {
		return nil, fmt.Errorf(
			"current sync staging plan differs from durable intent",
		)
	}
	files := []currentSyncStagedFile{}
	seen := map[string]bool{}
	add := func(path string, data []byte, mode uint32) error {
		if path == "" || mode == 0 {
			return fmt.Errorf("current sync staged file binding is invalid: %s", path)
		}
		key := strings.ToLower(path)
		if seen[key] {
			return fmt.Errorf("current sync staged file is duplicated: %s", path)
		}
		seen[key] = true
		files = append(files, currentSyncStagedFile{
			Path: path,
			Data: append([]byte(nil), data...),
			Mode: mode,
		})
		return nil
	}

	publications := map[string]currentSyncPublication{}
	for _, publication := range fresh.prepared.publications {
		path := ".steamai/" + publication.rel
		publications[strings.ToLower(path)] = publication
	}
	for _, root := range intent.Roots {
		for _, binding := range root.After.Entries {
			publication, ok := publications[strings.ToLower(binding.Path)]
			if !ok || currentSyncSHA(publication.data) != binding.SHA256 ||
				int64(len(publication.data)) != binding.Size ||
				uint32(publication.mode.Perm()) != binding.Mode {
				return nil, fmt.Errorf(
					"current sync staged controlled file differs from durable intent: %s",
					binding.Path,
				)
			}
			rel := strings.TrimPrefix(
				binding.Path,
				".steamai/"+root.Name+"/",
			)
			if rel == binding.Path || rel == "" {
				return nil, fmt.Errorf(
					"current sync staged controlled path is invalid: %s",
					binding.Path,
				)
			}
			if err := add(root.StagePath+"/"+rel, publication.data, binding.Mode); err != nil {
				return nil, err
			}
		}
	}

	leaves := map[string]currentSyncLeaf{}
	for _, leaf := range fresh.prepared.leaves {
		leaves[strings.ToLower(leaf.rel)] = leaf
	}
	for _, binding := range intent.Leaves {
		if !binding.Mutate || !binding.AfterExists {
			continue
		}
		leaf, ok := leaves[strings.ToLower(binding.Path)]
		if !ok || !leaf.afterExists ||
			currentSyncSHA(leaf.after) != binding.AfterSHA256 ||
			int64(len(leaf.after)) != binding.AfterSize ||
			uint32(leaf.mode.Perm()) != binding.Mode {
			return nil, fmt.Errorf(
				"current sync staged leaf differs from durable intent: %s",
				binding.Path,
			)
		}
		if err := add(binding.StagePath, leaf.after, binding.Mode); err != nil {
			return nil, err
		}
	}
	sort.Slice(files, func(left, right int) bool {
		return files[left].Path < files[right].Path
	})
	return files, nil
}
