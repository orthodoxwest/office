package review

import "testing"

func TestProvenanceQueueScoreWeightsFanout(t *testing.T) {
	e := ProvenanceQueueEntry{
		Occurrences: 10, PriorityAOccurrences: 3, PrincipalOccurrences: 4, DistinctCompositions: 2,
	}
	if got, want := provenanceQueueScore(e), 77; got != want {
		t.Fatalf("score = %d, want %d", got, want)
	}
}

func TestProvenanceQueueOrderingIsStable(t *testing.T) {
	high := ProvenanceQueueEntry{Key: "z", Score: 2}
	lowA := ProvenanceQueueEntry{Key: "a", Score: 1, Occurrences: 2}
	lowB := ProvenanceQueueEntry{Key: "b", Score: 1, Occurrences: 2}
	if !provenanceQueueLess(high, lowA) {
		t.Error("higher score should sort first")
	}
	if !provenanceQueueLess(lowA, lowB) {
		t.Error("key should break complete ties")
	}
}

func TestVerifiedQueueFilter(t *testing.T) {
	if shouldQueueProvenance(ProvenanceVerified, false) {
		t.Error("verified entry included by default")
	}
	if !shouldQueueProvenance(ProvenanceVerified, true) {
		t.Error("verified entry not included when requested")
	}
	if !shouldQueueProvenance(ProvenanceNeedsReview, false) {
		t.Error("pending entry excluded")
	}
}

func TestBuildProvenanceQueue(t *testing.T) {
	q, err := BuildProvenanceQueue("../../data", 2026, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Entries) == 0 {
		t.Fatal("empty queue")
	}
	if q.Entries[0].Score < q.Entries[len(q.Entries)-1].Score {
		t.Fatal("queue is not score-descending")
	}
	used, unused := false, false
	for _, e := range q.Entries {
		if e.ContentHash == "" {
			t.Fatalf("entry %s has no content hash", e.Key)
		}
		used = used || e.Occurrences > 0
		unused = unused || e.Occurrences == 0
	}
	if !used || !unused {
		t.Fatalf("used=%t unused=%t; queue should include every atomic entry", used, unused)
	}
}
