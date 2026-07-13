package main

import (
	"reflect"
	"testing"
)

func TestPlanPatchRolloverNoOpForCurrentPatch(t *testing.T) {
	plan, err := planPatchRollover("16.13", "16.13.1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CurrentPatch != "16.13" || plan.TargetPatch != "16.13" {
		t.Fatalf("plan patches = %+v, want current/target 16.13", plan)
	}
	if len(plan.ArchivePatches) != 0 {
		t.Fatalf("archive patches = %+v, want none", plan.ArchivePatches)
	}
}

func TestPlanPatchRolloverArchivesFallenPatch(t *testing.T) {
	plan, err := planPatchRollover("16.13", "16.14", 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"16.12"}
	if !reflect.DeepEqual(plan.ArchivePatches, want) {
		t.Fatalf("archive patches = %+v, want %+v", plan.ArchivePatches, want)
	}
}

func TestPlanPatchRolloverArchivesSkippedWindow(t *testing.T) {
	plan, err := planPatchRollover("16.13", "16.15", 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"16.13", "16.12"}
	if !reflect.DeepEqual(plan.ArchivePatches, want) {
		t.Fatalf("archive patches = %+v, want %+v", plan.ArchivePatches, want)
	}
}

func TestPlanPatchRolloverHandlesSeasonBoundary(t *testing.T) {
	plan, err := planPatchRollover("16.24", "17.1", 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"16.24", "16.23"}
	if !reflect.DeepEqual(plan.ArchivePatches, want) {
		t.Fatalf("archive patches = %+v, want %+v", plan.ArchivePatches, want)
	}
}

func TestPlanPatchRolloverIgnoresOlderTarget(t *testing.T) {
	plan, err := planPatchRollover("16.13", "16.12", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ArchivePatches) != 0 {
		t.Fatalf("archive patches = %+v, want none", plan.ArchivePatches)
	}
}

func TestPlanPatchRolloverRejectsInvalidCurrentPatch(t *testing.T) {
	if _, err := planPatchRollover("", "16.14", 2); err == nil {
		t.Fatal("expected invalid current patch error")
	}
}
