package review

import (
	"bytes"
	"strings"
	"testing"
)

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

func TestProvenanceQueueSuspectTierSortsFirst(t *testing.T) {
	suspect := ProvenanceQueueEntry{Key: "s", Score: 1, Flags: []Suspicion{{Label: "prescreen:high", State: SuspicionOpen}}}
	clean := ProvenanceQueueEntry{Key: "c", Score: 100}
	if !provenanceQueueLess(suspect, clean) {
		t.Error("suspect entry should outrank any clean entry")
	}
	if provenanceQueueLess(clean, suspect) {
		t.Error("clean entry sorted above suspect entry")
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
	seenClean := false
	prevScore := 0
	for i, e := range q.Entries {
		if e.Suspect() && seenClean {
			t.Fatalf("suspect entry %s ranked below a clean entry", e.Key)
		}
		if i > 0 && e.Suspect() == q.Entries[i-1].Suspect() && e.Score > prevScore {
			t.Fatalf("entry %s breaks score ordering within its tier", e.Key)
		}
		seenClean = seenClean || !e.Suspect()
		prevScore = e.Score
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

func TestProvenanceQueueCSVHidesContentHashes(t *testing.T) {
	q := &ProvenanceQueue{Entries: []ProvenanceQueueEntry{{
		Key: "psalms/001", ContentHash: "0123456789abcdef", Status: ProvenanceNeedsReview,
	}}}
	var out bytes.Buffer
	if err := WriteProvenanceQueueCSV(q, &out, "https://example.test"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "content_hash") || strings.Contains(out.String(), "0123456789abcdef") {
		t.Fatalf("reviewer queue exposes implementation hash:\n%s", out.String())
	}
}
