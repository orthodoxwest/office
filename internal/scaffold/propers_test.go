package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFeast(t *testing.T, dataDir, id, body string) {
	t.Helper()
	dir := filepath.Join(dataDir, "feasts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// LoadFeasts only reads the canonical feast files (see calendar.FeastFiles).
	path := filepath.Join(dir, "sanctoral.txt")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString("[" + id + "]\n" + body + "\n"); err != nil {
		t.Fatal(err)
	}
}

func TestEnsurePropersCreatesScaffold(t *testing.T) {
	dir := t.TempDir()
	writeFeast(t, dir, "st-ambrose", `Name = St. Ambrose
Rank = greater-double
Color = white
Category = confessor-doctor
Month = 12
Day = 7
`)

	results, err := EnsurePropers(dir, Options{Write: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	if results[0].Action != ActionCreate {
		t.Fatalf("Action = %s, want create", results[0].Action)
	}
	if !contains(results[0].AddedKeys, "collect") {
		t.Fatalf("AddedKeys missing collect: %v", results[0].AddedKeys)
	}

	path := filepath.Join(dir, "texts", "proper", "st-ambrose.txt")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "# [collect]") {
		t.Fatalf("scaffold missing # [collect]:\n%s", text)
	}
	if !strings.Contains(text, "# [benedictus-antiphon]") {
		t.Fatalf("scaffold missing # [benedictus-antiphon]")
	}
	if !strings.Contains(text, "# [chapter-lauds]") {
		t.Fatalf("scaffold missing optional # [chapter-lauds]")
	}
	// Must not create live empty sections.
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if liveSectionRE.MatchString(trimmed) {
			t.Fatalf("live section in new scaffold: %q", line)
		}
	}
	// Commons hint present.
	if !strings.Contains(text, "commons/confessor-doctor/") {
		t.Fatalf("expected commons hint, got:\n%s", text)
	}
}

func TestEnsurePropersAppendsWithoutTouchingLive(t *testing.T) {
	dir := t.TempDir()
	writeFeast(t, dir, "st-nicholas", `Name = St. Nicholas
Rank = double
Color = white
Category = confessor-bishop
Month = 12
Day = 6
`)
	properDir := filepath.Join(dir, "texts", "proper")
	if err := os.MkdirAll(properDir, 0o755); err != nil {
		t.Fatal(err)
	}
	original := `[collect]
# SOURCE: test
O God, who didst glorify the blessed Bishop Nicholas.

[benedictus-antiphon]
Benedictus of Nicholas.
`
	path := filepath.Join(properDir, "st-nicholas.txt")
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := EnsurePropers(dir, Options{Write: true})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Action != ActionAppend {
		t.Fatalf("Action = %s, want append", results[0].Action)
	}
	if contains(results[0].AddedKeys, "collect") {
		t.Fatalf("must not re-add live collect: %v", results[0].AddedKeys)
	}
	if contains(results[0].AddedKeys, "benedictus-antiphon") {
		t.Fatalf("must not re-add live benedictus-antiphon: %v", results[0].AddedKeys)
	}
	if !contains(results[0].AddedKeys, "magnificat-antiphon") {
		t.Fatalf("should add missing magnificat-antiphon: %v", results[0].AddedKeys)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	// Live content preserved at start.
	if !strings.HasPrefix(text, "[collect]\n# SOURCE: test\nO God, who didst glorify the blessed Bishop Nicholas.\n") {
		t.Fatalf("live collect mutated:\n%s", text)
	}
	if !strings.Contains(text, "[benedictus-antiphon]\nBenedictus of Nicholas.\n") {
		t.Fatalf("live benedictus mutated:\n%s", text)
	}
	if !strings.Contains(text, "# [magnificat-antiphon]") {
		t.Fatalf("missing appended magnificat scaffold:\n%s", text)
	}
	// collect must not appear as commented duplicate.
	if strings.Contains(text, "# [collect]") {
		t.Fatalf("commented duplicate of live collect:\n%s", text)
	}
}

func TestEnsurePropersIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeFeast(t, dir, "st-ambrose", `Name = St. Ambrose
Rank = greater-double
Color = white
Category = confessor-doctor
Month = 12
Day = 7
`)

	if _, err := EnsurePropers(dir, Options{Write: true}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "texts", "proper", "st-ambrose.txt")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	results, err := EnsurePropers(dir, Options{Write: true})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Action != ActionOK {
		t.Fatalf("second run Action = %s, want ok", results[0].Action)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("second run rewrote file")
	}
}

func TestEnsurePropersSkipsCommemorationsByDefault(t *testing.T) {
	dir := t.TempDir()
	writeFeast(t, dir, "comm-st-foo", `Name = St. Foo
Rank = commemoration
Color = white
Category = confessor
Month = 1
Day = 1
`)

	results, err := EnsurePropers(dir, Options{Write: true})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Action != ActionSkip {
		t.Fatalf("Action = %s, want skip", results[0].Action)
	}
	if _, err := os.Stat(filepath.Join(dir, "texts", "proper", "comm-st-foo.txt")); !os.IsNotExist(err) {
		t.Fatalf("commemoration file should not exist, err=%v", err)
	}
}

func TestEnsurePropersIncludeCommemorations(t *testing.T) {
	dir := t.TempDir()
	writeFeast(t, dir, "comm-st-foo", `Name = St. Foo
Rank = commemoration
Color = white
Category = confessor
Month = 1
Day = 1
`)

	results, err := EnsurePropers(dir, Options{Write: true, IncludeCommemorations: true})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Action != ActionCreate {
		t.Fatalf("Action = %s, want create", results[0].Action)
	}
	body, err := os.ReadFile(filepath.Join(dir, "texts", "proper", "comm-st-foo.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "# [commemoration-antiphon]") {
		t.Fatalf("expected thin catalog:\n%s", text)
	}
	if strings.Contains(text, "# [chapter-lauds]") {
		t.Fatalf("commemoration scaffold should not include optional chapter-lauds")
	}
}

func TestEnsurePropersDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	writeFeast(t, dir, "st-ambrose", `Name = St. Ambrose
Rank = greater-double
Color = white
Category = confessor-doctor
Month = 12
Day = 7
`)

	results, err := EnsurePropers(dir, Options{Write: false})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Action != ActionCreate {
		t.Fatalf("Action = %s, want create", results[0].Action)
	}
	if _, err := os.Stat(filepath.Join(dir, "texts", "proper", "st-ambrose.txt")); !os.IsNotExist(err) {
		t.Fatalf("dry-run must not create file")
	}
	if !NeedsWork(results) {
		t.Fatal("NeedsWork = false, want true")
	}
}

func TestEnsurePropersRecognizesExistingCommentedKeys(t *testing.T) {
	dir := t.TempDir()
	writeFeast(t, dir, "st-foo", `Name = St. Foo
Rank = double
Color = white
Category = martyr
Month = 1
Day = 2
`)
	properDir := filepath.Join(dir, "texts", "proper")
	if err := os.MkdirAll(properDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Sparse live + already-scaffolded comment for magnificat.
	content := `[collect]
A collect.

# [magnificat-antiphon]
# already scaffolded
`
	if err := os.WriteFile(filepath.Join(properDir, "st-foo.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := EnsurePropers(dir, Options{Write: true})
	if err != nil {
		t.Fatal(err)
	}
	if contains(results[0].AddedKeys, "magnificat-antiphon") {
		t.Fatalf("must not re-add already-commented magnificat: %v", results[0].AddedKeys)
	}
	if contains(results[0].AddedKeys, "collect") {
		t.Fatalf("must not re-add live collect")
	}
	if !contains(results[0].AddedKeys, "benedictus-antiphon") {
		t.Fatalf("should still add benedictus: %v", results[0].AddedKeys)
	}

	// Second run ok.
	results, err = EnsurePropers(dir, Options{Write: true})
	if err != nil {
		t.Fatal(err)
	}
	// Still may append remaining keys from first run only once — second should be ok
	// if first completed catalog.
	if results[0].Action != ActionOK {
		// first run should have filled all remaining; if not, fail with detail
		t.Fatalf("second Action = %s keys=%v", results[0].Action, results[0].AddedKeys)
	}
}

func TestParsePresentKeys(t *testing.T) {
	s := `[collect]
text

# [benedictus-antiphon]
# blurb

# note not a key
[magnificat-antiphon]
more
`
	live, commented := parsePresentKeys(&s)
	if !live["collect"] || !live["magnificat-antiphon"] {
		t.Fatalf("live = %v", live)
	}
	if live["benedictus-antiphon"] {
		t.Fatal("commented key should not be live")
	}
	if !commented["benedictus-antiphon"] {
		t.Fatalf("commented = %v", commented)
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
