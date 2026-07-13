package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/clickhouse"
	"winrift/core/internal/staticdata"
)

type patchRolloverPlan struct {
	CurrentPatch   string
	TargetPatch    string
	ArchivePatches []string
}

func latestDataDragonPatchBucket(ctx context.Context, staticService *staticdata.Service) (string, error) {
	latestVersion, err := staticService.LatestVersion(ctx)
	if err != nil {
		return "", err
	}
	return normalizePatchBucket(latestVersion)
}

func rolloverPatchWindow(
	ctx context.Context,
	repo *clickhouse.Repository,
	staticService *staticdata.Service,
	targetPatch string,
	currentPatch string,
	retentionCount int,
	platform string,
	queueID uint16,
	retainedUntil time.Time,
	pruneRaw bool,
) error {
	targetPatch = strings.TrimSpace(targetPatch)
	var err error
	if targetPatch == "" {
		targetPatch, err = latestDataDragonPatchBucket(ctx, staticService)
		if err != nil {
			return fmt.Errorf("latest patch: %w", err)
		}
	}

	plan, err := planPatchRollover(currentPatch, targetPatch, retentionCount)
	if err != nil {
		return err
	}
	if len(plan.ArchivePatches) == 0 {
		log.Printf("patch rollover no-op current_patch=%s target_patch=%s retention=%d", plan.CurrentPatch, plan.TargetPatch, normalizedRetention(retentionCount))
		return nil
	}

	log.Printf(
		"patch rollover start current_patch=%s target_patch=%s retention=%d archive_patches=%s",
		plan.CurrentPatch,
		plan.TargetPatch,
		normalizedRetention(retentionCount),
		strings.Join(plan.ArchivePatches, ","),
	)
	for _, patch := range plan.ArchivePatches {
		if err := archivePatchIfStored(ctx, repo, staticService, patch, platform, queueID, retainedUntil, pruneRaw); err != nil {
			return err
		}
	}
	log.Printf("patch rollover complete target_patch=%s", plan.TargetPatch)
	return nil
}

func archivePatchIfStored(
	ctx context.Context,
	repo *clickhouse.Repository,
	staticService *staticdata.Service,
	patch string,
	platform string,
	queueID uint16,
	retainedUntil time.Time,
	pruneRaw bool,
) error {
	if strings.EqualFold(strings.TrimSpace(platform), "ALL") {
		platforms, err := repo.PatchPlatforms(ctx, patch, queueID)
		if err != nil {
			return err
		}
		if len(platforms) == 0 {
			log.Printf("patch rollover skip archive patch=%s queue=%d reason=no stored platforms", patch, queueID)
			return nil
		}
	}
	return archivePatch(ctx, repo, staticService, patch, platform, queueID, retainedUntil, pruneRaw)
}

func planPatchRollover(currentPatch, targetPatch string, retentionCount int) (patchRolloverPlan, error) {
	currentPatch, err := normalizePatchBucket(currentPatch)
	if err != nil {
		return patchRolloverPlan{}, fmt.Errorf("current patch: %w", err)
	}
	targetPatch, err = normalizePatchBucket(targetPatch)
	if err != nil {
		return patchRolloverPlan{}, fmt.Errorf("target patch: %w", err)
	}

	comparison, err := comparePatchBuckets(targetPatch, currentPatch)
	if err != nil {
		return patchRolloverPlan{}, err
	}
	if comparison <= 0 {
		return patchRolloverPlan{CurrentPatch: currentPatch, TargetPatch: targetPatch}, nil
	}

	currentWindow := analytics.PatchWindow(currentPatch, retentionCount)
	targetWindow := map[string]bool{}
	for _, patch := range analytics.PatchWindow(targetPatch, retentionCount) {
		targetWindow[patch] = true
	}

	archivePatches := make([]string, 0, len(currentWindow))
	for _, patch := range currentWindow {
		if !targetWindow[patch] {
			archivePatches = append(archivePatches, patch)
		}
	}
	return patchRolloverPlan{
		CurrentPatch:   currentPatch,
		TargetPatch:    targetPatch,
		ArchivePatches: archivePatches,
	}, nil
}

func normalizePatchBucket(version string) (string, error) {
	patch := analytics.PatchBucket(strings.TrimSpace(version))
	if _, _, ok := parseComparablePatchBucket(patch); !ok {
		return "", fmt.Errorf("invalid patch bucket %q", version)
	}
	return patch, nil
}

func comparePatchBuckets(left, right string) (int, error) {
	leftMajor, leftMinor, ok := parseComparablePatchBucket(left)
	if !ok {
		return 0, fmt.Errorf("invalid patch bucket %q", left)
	}
	rightMajor, rightMinor, ok := parseComparablePatchBucket(right)
	if !ok {
		return 0, fmt.Errorf("invalid patch bucket %q", right)
	}
	switch {
	case leftMajor < rightMajor:
		return -1, nil
	case leftMajor > rightMajor:
		return 1, nil
	case leftMinor < rightMinor:
		return -1, nil
	case leftMinor > rightMinor:
		return 1, nil
	default:
		return 0, nil
	}
}

func parseComparablePatchBucket(patch string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(patch), ".")
	if len(parts) != 2 {
		return 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	if major <= 0 || minor <= 0 {
		return 0, 0, false
	}
	return major, minor, true
}

func normalizedRetention(retentionCount int) int {
	if retentionCount <= 0 {
		return 1
	}
	return retentionCount
}
