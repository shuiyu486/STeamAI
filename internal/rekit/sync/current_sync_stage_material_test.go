package sync

import (
	"strings"
	"testing"
)

func TestCurrentSyncStageMaterialMatchesDurableIntentExactly(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	opt := CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "refreshed-project",
		SourceExecutable: fixture.sourceExecutable,
	}
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		opt,
	)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}
	files, err := currentSyncStageMaterial(intent, plan)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]currentSyncStagedFile{}
	for _, file := range files {
		if _, exists := got[strings.ToLower(file.Path)]; exists {
			t.Fatalf("current sync staged file is duplicated: %s", file.Path)
		}
		got[strings.ToLower(file.Path)] = file
	}
	for _, root := range intent.Roots {
		for _, binding := range root.After.Entries {
			rel := strings.TrimPrefix(
				binding.Path,
				".steamai/"+root.Name+"/",
			)
			key := strings.ToLower(root.StagePath + "/" + rel)
			file, exists := got[key]
			if !exists || currentSyncSHA(file.Data) != binding.SHA256 ||
				int64(len(file.Data)) != binding.Size ||
				file.Mode != binding.Mode {
				t.Fatalf("current sync staged root file differs: binding=%+v file=%+v", binding, file)
			}
			delete(got, key)
		}
	}
	for _, leaf := range intent.Leaves {
		key := strings.ToLower(leaf.StagePath)
		file, exists := got[key]
		if leaf.Mutate && leaf.AfterExists {
			if !exists || currentSyncSHA(file.Data) != leaf.AfterSHA256 ||
				int64(len(file.Data)) != leaf.AfterSize ||
				file.Mode != leaf.Mode {
				t.Fatalf("current sync staged leaf differs: leaf=%+v file=%+v", leaf, file)
			}
			delete(got, key)
		} else if exists {
			t.Fatalf("non-published current sync leaf was staged: %+v", leaf)
		}
	}
	if len(got) != 0 {
		t.Fatalf("current sync staged unbound files: %+v", got)
	}
}

func TestCurrentSyncStageMaterialRejectsPreparedDrift(t *testing.T) {
	fixture := newCurrentSyncFixture(t, "")
	opt := CurrentSyncOptions{
		Command:          "sync",
		ProjectName:      "refreshed-project",
		SourceExecutable: fixture.sourceExecutable,
	}
	plan, err := CurrentSyncPreview(
		fixture.repoRoot,
		fixture.caseRoot,
		fixture.pack,
		opt,
	)
	if err != nil {
		t.Fatal(err)
	}
	intent, err := buildCurrentSyncIntent(plan)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("plan-identity", func(t *testing.T) {
		drift := plan
		drift.ProjectName = "drifted-project"
		if _, err := currentSyncStageMaterial(intent, drift); err == nil ||
			!strings.Contains(err.Error(), "plan differs") {
			t.Fatalf("current sync accepted staging plan drift: %v", err)
		}
	})

	t.Run("controlled-bytes", func(t *testing.T) {
		drift := cloneCurrentSyncPreparedPlan(plan)
		index := -1
		for candidate := range drift.prepared.publications {
			path := ".steamai/" + drift.prepared.publications[candidate].rel
			for _, root := range intent.Roots {
				if strings.HasPrefix(path, ".steamai/"+root.Name+"/") {
					index = candidate
					break
				}
			}
			if index >= 0 {
				break
			}
		}
		if index < 0 {
			t.Fatal("current sync fixture has no mutating controlled publication")
		}
		drift.prepared.publications[index].data = append(
			drift.prepared.publications[index].data,
			byte('x'),
		)
		if _, err := currentSyncStageMaterial(intent, drift); err == nil ||
			!strings.Contains(err.Error(), "controlled file differs") {
			t.Fatalf("current sync accepted staged controlled drift: %v", err)
		}
	})

	t.Run("leaf-bytes", func(t *testing.T) {
		drift := cloneCurrentSyncPreparedPlan(plan)
		index := -1
		for candidate, leaf := range drift.prepared.leaves {
			for _, binding := range intent.Leaves {
				if binding.Mutate && binding.AfterExists &&
					strings.EqualFold(binding.Path, leaf.rel) {
					index = candidate
					break
				}
			}
			if index >= 0 {
				break
			}
		}
		if index < 0 {
			t.Fatal("current sync fixture has no mutating staged leaf")
		}
		drift.prepared.leaves[index].after = append(
			drift.prepared.leaves[index].after,
			byte('x'),
		)
		if _, err := currentSyncStageMaterial(intent, drift); err == nil ||
			!strings.Contains(err.Error(), "staged leaf differs") {
			t.Fatalf("current sync accepted staged leaf drift: %v", err)
		}
	})
}

func cloneCurrentSyncPreparedPlan(plan CurrentSyncPlan) CurrentSyncPlan {
	clone := plan
	prepared := *plan.prepared
	prepared.publications = append(
		[]currentSyncPublication(nil),
		plan.prepared.publications...,
	)
	for index := range prepared.publications {
		prepared.publications[index].data = append(
			[]byte(nil),
			prepared.publications[index].data...,
		)
	}
	prepared.leaves = append([]currentSyncLeaf(nil), plan.prepared.leaves...)
	for index := range prepared.leaves {
		prepared.leaves[index].before = append(
			[]byte(nil),
			prepared.leaves[index].before...,
		)
		prepared.leaves[index].after = append(
			[]byte(nil),
			prepared.leaves[index].after...,
		)
	}
	clone.prepared = &prepared
	return clone
}
