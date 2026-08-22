package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const siblingCollect = `[collect]
# SOURCE: original collect
Almighty God, original collect text.
`

const hymnScaffold = `# [hymn-lauds]
# Hymn at Lauds. First line may be a Latin incipit title.
#
`

func TestReplaceOrInsertLeavesSiblingLiveSection(t *testing.T) {
	original := siblingCollect + "\n[hymn-lauds]\nOld hymn.\n"
	got, err := replaceOrInsertSection(original, "hymn-lauds", "monastic-diurnal p. 80 (DI-x; hymn-lauds) — "+SourceHedge, "New hymn body.")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, siblingCollect) {
		t.Fatalf("sibling collect changed:\n%s", got)
	}
	if strings.Contains(got, "Old hymn") {
		t.Fatalf("old hymn remained:\n%s", got)
	}
	if !strings.Contains(got, "New hymn body.") || !strings.Contains(got, SourceHedge) {
		t.Fatalf("new section missing:\n%s", got)
	}
}

func TestReplaceOrInsertUncommentsScaffold(t *testing.T) {
	original := siblingCollect + hymnScaffold
	got, err := replaceOrInsertSection(original, "hymn-lauds", "monastic-diurnal p. 80 (DI-x) — "+SourceHedge, "Gem of the highest.")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "# [hymn-lauds]") {
		t.Fatalf("commented header remained:\n%s", got)
	}
	if !strings.Contains(got, "[hymn-lauds]\n# SOURCE:") {
		t.Fatalf("live header missing:\n%s", got)
	}
	if !strings.HasPrefix(got, siblingCollect) {
		t.Fatalf("collect prefix lost:\n%s", got)
	}
}

func TestReplaceOrInsertRefusesEmptyBody(t *testing.T) {
	_, err := writeTargetContent(siblingCollect, applyTarget{family: "proper", slot: "collect"}, "c — "+SourceHedge, "  \n")
	if err == nil {
		t.Fatal("expected empty-body error")
	}
}

func TestApplyQueueWritesGatedAdd(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "proper", "st-benedict.txt"), siblingCollect+hymnScaffold)
	writeTestFile(t, filepath.Join(dir, "review", "provenance.csv"), strings.Join(provenanceHeader, ",")+"\n")

	packet := validPacket()
	packet.TargetKey = "proper/st-benedict/hymn-lauds"
	setPacketBody(&packet, "Laudibus cives resonent canoris\n\nGem of the highest.")
	packet.SourceComment = FormatSourceComment("monastic-diurnal", packet.Pages, packet.CandidateID, "hymn-lauds")

	report, err := ApplyQueue(dir, ApplyQueueFile{Packets: []ApplyPacket{packet}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Wrote != 1 || report.Allowed != 1 {
		t.Fatalf("report = %+v", report)
	}

	got, err := os.ReadFile(filepath.Join(dir, "texts", "proper", "st-benedict.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, siblingCollect) {
		t.Fatalf("sibling lost:\n%s", text)
	}
	if !strings.Contains(text, "[hymn-lauds]") || strings.Contains(text, "# [hymn-lauds]") {
		t.Fatalf("hymn not live:\n%s", text)
	}
	if !strings.Contains(text, SourceHedge) {
		t.Fatalf("missing hedge:\n%s", text)
	}

	prov, err := os.ReadFile(filepath.Join(dir, "review", "provenance.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(prov)) != strings.Join(provenanceHeader, ",") {
		t.Fatalf("provenance.csv changed: %q", prov)
	}
}

func TestApplyQueueDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	original := siblingCollect + hymnScaffold
	writeTestFile(t, filepath.Join(dir, "texts", "proper", "st-benedict.txt"), original)
	writeTestFile(t, filepath.Join(dir, "review", "provenance.csv"), strings.Join(provenanceHeader, ",")+"\n")

	packet := validPacket()
	packet.TargetKey = "proper/st-benedict/hymn-lauds"
	setPacketBody(&packet, "Gem of the highest.")
	packet.SourceComment = "monastic-diurnal p. 80 (DI-x) — " + SourceHedge

	report, err := ApplyQueue(dir, ApplyQueueFile{Packets: []ApplyPacket{packet}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if report.Allowed != 1 || report.Wrote != 0 {
		t.Fatalf("dry-run report = %+v", report)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "texts", "proper", "st-benedict.txt"))
	if string(got) != original {
		t.Fatalf("dry-run mutated file:\n%s", got)
	}
}

func TestApplyQueueRefusesPsalmPacket(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "psalms", "067.txt"), "Let God arise.\n")
	writeTestFile(t, filepath.Join(dir, "texts", "proper", "st-benedict.txt"), siblingCollect)
	writeTestFile(t, filepath.Join(dir, "review", "provenance.csv"), strings.Join(provenanceHeader, ",")+"\n")

	packet := validPacket()
	packet.TargetKey = "psalms/067"
	packet.Action = ApplyReplaceSection
	packet.DiscoveryClass = ClassExistingDifferent
	packet.TextSimilarity = 0.9
	setPacketBody(&packet, "Let God arise, and let his enemies be scattered.")

	_, err := ApplyQueue(dir, ApplyQueueFile{Packets: []ApplyPacket{packet}}, false)
	if err == nil {
		t.Fatal("expected refuse")
	}
	got, _ := os.ReadFile(filepath.Join(dir, "texts", "psalms", "067.txt"))
	if string(got) != "Let God arise.\n" {
		t.Fatalf("psalm was written: %q", got)
	}
}

func TestApplyQueueNoopDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "proper", "st-benedict.txt"), siblingCollect)
	writeTestFile(t, filepath.Join(dir, "review", "provenance.csv"), strings.Join(provenanceHeader, ",")+"\n")

	packet := validPacket()
	packet.TargetKey = "proper/st-benedict/collect"
	packet.Action = ApplyReplaceSection
	packet.DiscoveryClass = ClassFallbackEqual
	packet.TextSimilarity = 0.99
	setPacketBody(&packet, "Almighty God, original collect text.")
	packet.SourceComment = "x — " + SourceHedge

	report, err := ApplyQueue(dir, ApplyQueueFile{Packets: []ApplyPacket{packet}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if report.Noop != 1 || report.Wrote != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestApplyQueueRefusesWitnessBodyMismatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "texts", "proper", "st-benedict.txt"), siblingCollect+hymnScaffold)
	writeTestFile(t, filepath.Join(dir, "review", "provenance.csv"), strings.Join(provenanceHeader, ",")+"\n")

	packet := validPacket()
	packet.TargetKey = "proper/st-benedict/hymn-lauds"
	packet.Body = "Laudibus cives resonent canoris."
	report, err := ApplyQueue(dir, ApplyQueueFile{Packets: []ApplyPacket{packet}}, true)
	if err == nil {
		t.Fatal("expected witness/body mismatch refusal")
	}
	if report.Refused != 1 || report.Allowed != 0 || report.Wrote != 0 {
		t.Fatalf("report = %+v", report)
	}
}

func TestConfinedTextPathRejectsEscape(t *testing.T) {
	if _, err := confinedTextPath("/data", "texts/../review/provenance.csv"); err == nil {
		t.Fatal("expected escape error")
	}
	if _, err := confinedTextPath("/data", "texts/psalms/001.txt"); err == nil {
		t.Fatal("expected psalm error")
	}
}

func TestRelativeTextPath(t *testing.T) {
	got, err := relativeTextPath("proper/st-benedict/chapter-lauds")
	if err != nil || got != "texts/proper/st-benedict.txt" {
		t.Fatalf("proper = %q %v", got, err)
	}
	got, err = relativeTextPath("canticles/benedictus")
	if err != nil || got != "texts/canticles/benedictus.txt" {
		t.Fatalf("canticle = %q %v", got, err)
	}
	got, err = relativeTextPath("ordinary/marian/alma-redemptoris-advent")
	if err != nil || got != "texts/ordinary/marian/alma-redemptoris-advent.txt" {
		t.Fatalf("marian = %q %v", got, err)
	}
	got, err = relativeTextPath("shared/formulas/collect-conclusion-long")
	if err != nil || got != "texts/shared/formulas.txt" {
		t.Fatalf("shared = %q %v", got, err)
	}
}
