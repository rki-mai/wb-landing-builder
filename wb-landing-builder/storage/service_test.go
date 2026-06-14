package storage

import (
	"slices"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func elementMutation(id string) bson.M {
	return bson.M{
		"id":      id,
		"deleted": false,
	}
}

func revertMutation(count int) bson.M {
	return bson.M{"revert": count}
}

func elementIDs(mutations []bson.M) []string {
	ids := make([]string, 0, len(mutations))
	for _, mutation := range mutations {
		id, ok := mutation["id"].(string)
		if !ok {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

func assertEquivalent(t *testing.T, history []bson.M, expected []bson.M) {
	t.Helper()
	got := dedupElementMutations(resolveMutations(history))
	gotIDs := elementIDs(got)
	expectedIDs := elementIDs(expected)
	slices.Sort(gotIDs)
	slices.Sort(expectedIDs)
	if len(gotIDs) != len(expectedIDs) {
		t.Fatalf("expected IDs %v, got %v (full: %+v)", expectedIDs, gotIDs, got)
	}
	for i := range expectedIDs {
		if gotIDs[i] != expectedIDs[i] {
			t.Fatalf("expected IDs %v, got %v", expectedIDs, gotIDs)
		}
	}
}

func TestResolveMutationsIssueExamples(t *testing.T) {
	mut1 := elementMutation("lb-1")
	mut2 := elementMutation("lb-2")
	mut3 := elementMutation("lb-3")
	mut4 := elementMutation("lb-4")

	tests := []struct {
		name     string
		history  []bson.M
		expected []bson.M
	}{
		{
			// issue: [revert(2), mut1, mut2, mut3] → v1=mut3 … v4=revert(2)
			name:     "revert(2) undoes two preceding mutations",
			history:  []bson.M{mut3, mut2, mut1, revertMutation(2)},
			expected: []bson.M{mut3},
		},
		{
			// issue: [revert(1), revert(2), mut1, mut2]
			name:     "revert cancels preceding revert",
			history:  []bson.M{mut2, mut1, revertMutation(2), revertMutation(1)},
			expected: []bson.M{mut1, mut2},
		},
		{
			// issue: [revert(3), revert(1), mut1, mut2, mut3]
			name:     "revert(3) undoes revert and two element mutations",
			history:  []bson.M{mut3, mut2, mut1, revertMutation(1), revertMutation(3)},
			expected: []bson.M{mut3},
		},
		{
			// issue: [revert(1), mut1, mut2, revert(1), mut3, mut4]
			name:     "two reverts undo non-adjacent mutations",
			history:  []bson.M{mut4, mut3, revertMutation(1), mut2, mut1, revertMutation(1)},
			expected: []bson.M{mut2, mut4},
		},
		{
			// issue: [mut1, revert(1), mut2, mut3]
			name:     "revert in the middle of history",
			history:  []bson.M{mut3, mut2, revertMutation(1), mut1},
			expected: []bson.M{mut1, mut3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertEquivalent(t, tt.history, tt.expected)
		})
	}
}

func TestResolveMutationsEdgeCases(t *testing.T) {
	mut1 := elementMutation("lb-1")

	t.Run("revert on empty history is no-op", func(t *testing.T) {
		got := resolveMutations([]bson.M{revertMutation(1)})
		if len(got) != 0 {
			t.Fatalf("expected empty survivors, got %v", got)
		}
	})

	t.Run("revert count greater than available preceding mutations clamps silently", func(t *testing.T) {
		assertEquivalent(t, []bson.M{mut1, revertMutation(5)}, []bson.M{})
	})

	t.Run("deleted element survives dedup when last active record", func(t *testing.T) {
		deleted := bson.M{"id": "lb-1", "deleted": true}
		got := dedupElementMutations(resolveMutations([]bson.M{mut1, deleted}))
		if len(got) != 0 {
			t.Fatalf("expected deleted element to be removed, got %v", got)
		}
	})

	t.Run("inactive revert does not remove mutations", func(t *testing.T) {
		history := []bson.M{elementMutation("lb-2"), elementMutation("lb-1"), revertMutation(1), revertMutation(1)}
		assertEquivalent(t, history, []bson.M{elementMutation("lb-1"), elementMutation("lb-2")})
	})

	t.Run("revert after update undoes preceding mutation", func(t *testing.T) {
		original := bson.M{"id": "lb-2", "deleted": false, "value": "original"}
		updated := bson.M{"id": "lb-2", "deleted": false, "value": "updated"}
		got := dedupElementMutations(resolveMutations([]bson.M{original, updated, revertMutation(1)}))
		if len(got) != 1 {
			t.Fatalf("expected 1 element, got %d", len(got))
		}
		if got[0]["value"] != "original" {
			t.Fatalf("expected original value, got %v", got[0]["value"])
		}
	})

	t.Run("delete after revert is not undone by revert", func(t *testing.T) {
		creates := []bson.M{
			{"id": "lb-1", "deleted": false},
			{"id": "lb-2", "deleted": false, "value": "original"},
			{"id": "lb-5", "deleted": false},
		}
		updated := bson.M{"id": "lb-2", "deleted": false, "value": "updated"}
		deleted := bson.M{"id": "lb-5", "deleted": true}
		history := append(append([]bson.M{}, creates...), updated, revertMutation(1), deleted)
		got := dedupElementMutations(resolveMutations(history))
		gotIDs := elementIDs(got)
		slices.Sort(gotIDs)
		if slices.Contains(gotIDs, "lb-5") {
			t.Fatalf("expected lb-5 to be deleted, got %v", gotIDs)
		}
		if len(gotIDs) != 2 || gotIDs[0] != "lb-1" || gotIDs[1] != "lb-2" {
			t.Fatalf("expected lb-1 and lb-2, got %v", gotIDs)
		}
		for _, mutation := range got {
			if mutation["id"] == "lb-2" && mutation["value"] != "original" {
				t.Fatalf("expected reverted heading, got %v", mutation["value"])
			}
		}
	})

	t.Run("revert after delete restores element", func(t *testing.T) {
		created := bson.M{"id": "lb-5", "deleted": false}
		deleted := bson.M{"id": "lb-5", "deleted": true}
		got := dedupElementMutations(resolveMutations([]bson.M{created, deleted, revertMutation(1)}))
		gotIDs := elementIDs(got)
		if len(gotIDs) != 1 || gotIDs[0] != "lb-5" {
			t.Fatalf("expected lb-5 restored, got %v", gotIDs)
		}
	})
}

func TestRevertCountFromData(t *testing.T) {
	count, err := revertCountFromData(bson.M{"count": float64(2)})
	if err != nil || count != 2 {
		t.Fatalf("expected count 2, got %d err=%v", count, err)
	}

	_, err = revertCountFromData(bson.M{"count": float64(0)})
	if err != ErrInvalidMutation {
		t.Fatalf("expected ErrInvalidMutation, got %v", err)
	}
}
