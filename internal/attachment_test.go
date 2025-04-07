package internal

import (
	"slices"
	"testing"
)

func TestAttachmentDiff(t *testing.T) {
	postIds := []string{
		"2ce11ca3-654b-4afb-ab31-bb18ca46a3b8",
		"1a8dd676-1bc1-4a6d-95bf-6f2e8e121313",
	}

	knownIds := []string{
		"2ce11ca3-654b-4afb-ab31-bb18ca46a3b8",
		"769cb3a3-ba08-43ff-8ee8-54bed8dea63d",
		"070d2e4f-1890-4001-ac32-107edbf492ff",
	}

	diffs := DiffAttachments(postIds, knownIds)

	if !slices.Contains(diffs.ToAdd, knownIds[0]) {
	}

}
