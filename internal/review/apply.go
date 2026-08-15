package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/orthodoxwest/office/internal/calendar"
	"github.com/orthodoxwest/office/internal/scaffold"
	"github.com/orthodoxwest/office/internal/texts"
)

// ApplyQueueFile is the JSON document `office review apply` reads.
type ApplyQueueFile struct {
	Packets []ApplyPacket `json:"packets"`
}

// ApplyItemResult is the per-packet report from ApplyQueue.
type ApplyItemResult struct {
	CandidateID string
	TargetKey   string
	Decision    GateDecision
	Path        string // relative to dataDir when a write (or dry-run write) is planned
}

// ApplyReport is the queue-level result.
type ApplyReport struct {
	Items   []ApplyItemResult
	Allowed int
	Noop    int
	Refused int
	Wrote   int
}

var (
	liveSectionLine      = regexp.MustCompile(`^\[([a-z0-9-]+)\]\s*$`)
	commentedSectionLine = regexp.MustCompile(`^#\s*\[([a-z0-9-]+)\]\s*$`)
)

// LoadApplyWorld builds the gate's corpus/calendar snapshot from data/.
func LoadApplyWorld(dataDir string) (ApplyWorld, error) {
	world := ApplyWorld{
		ExistingKeys: map[string]bool{},
		KnownOwners:  map[string]bool{"canticles": true},
		VerifiedKeys: map[string]bool{},
	}
	corpus, err := texts.LoadTexts(dataDir)
	if err != nil {
		return world, fmt.Errorf("loading texts: %w", err)
	}
	for key := range corpus.Entries() {
		world.ExistingKeys[key] = true
		if parsed, err := parseApplyTarget(key); err == nil && parsed.ownerKey != "" {
			world.KnownOwners[parsed.ownerKey] = true
		}
	}
	if feasts, err := calendar.LoadFeasts(dataDir); err == nil {
		for _, feast := range feasts {
			world.KnownOwners["proper/"+feast.ID] = true
		}
	}
	inv, err := ScanProvenance(dataDir)
	if err != nil {
		return world, fmt.Errorf("scanning provenance: %w", err)
	}
	for _, entry := range inv.Entries {
		if entry.Status == ProvenanceVerified && !entry.Stale {
			world.VerifiedKeys[entry.Key] = true
		}
	}
	return world, nil
}

// LoadApplyQueue reads a packets object (or a bare array) from path.
func LoadApplyQueue(path string) (ApplyQueueFile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ApplyQueueFile{}, err
	}
	var queue ApplyQueueFile
	if err := json.Unmarshal(raw, &queue); err == nil && queue.Packets != nil {
		return queue, nil
	}
	var packets []ApplyPacket
	if err := json.Unmarshal(raw, &packets); err != nil {
		return ApplyQueueFile{}, fmt.Errorf("apply queue must be an object with packets or an array: %w", err)
	}
	return ApplyQueueFile{Packets: packets}, nil
}

// ApplyQueue gates each packet and, unless dryRun, writes allowed packets
// into data/texts. It never writes provenance, signoffs, psalms, or feasts.
func ApplyQueue(dataDir string, queue ApplyQueueFile, dryRun bool) (ApplyReport, error) {
	world, err := LoadApplyWorld(dataDir)
	if err != nil {
		return ApplyReport{}, err
	}
	report := ApplyReport{}
	var allowed []ApplyPacket
	for _, packet := range queue.Packets {
		decision := Gate(packet, world)
		item := ApplyItemResult{
			CandidateID: packet.CandidateID,
			TargetKey:   packet.TargetKey,
			Decision:    decision,
		}
		switch decision.Status {
		case GateAllow:
			report.Allowed++
			rel, err := relativeTextPath(packet.TargetKey)
			if err != nil {
				decision.Status = GateRefuse
				decision.Reasons = append(decision.Reasons, err.Error())
				item.Decision = decision
				report.Allowed--
				report.Refused++
			} else {
				item.Path = rel
				allowed = append(allowed, packet)
			}
		case GateNoop:
			report.Noop++
		default:
			report.Refused++
		}
		report.Items = append(report.Items, item)
	}
	if report.Refused > 0 {
		return report, fmt.Errorf("apply queue has %d refused packet(s)", report.Refused)
	}
	if dryRun {
		return report, nil
	}
	for _, packet := range allowed {
		if err := applyPacket(dataDir, packet); err != nil {
			return report, err
		}
		report.Wrote++
		// Subsequent packets in the same queue must see the new key.
		world.ExistingKeys[packet.TargetKey] = true
	}
	return report, nil
}

func applyPacket(dataDir string, packet ApplyPacket) error {
	rel, err := relativeTextPath(packet.TargetKey)
	if err != nil {
		return err
	}
	abs, err := confinedTextPath(dataDir, rel)
	if err != nil {
		return err
	}
	parsed, err := parseApplyTarget(packet.TargetKey)
	if err != nil {
		return err
	}
	comment := packet.SourceComment
	if comment == "" {
		comment = FormatSourceComment("monastic-diurnal", packet.Pages, packet.CandidateID, parsed.slot)
	}
	if !strings.Contains(comment, SourceHedge) {
		return fmt.Errorf("source comment must include %q", SourceHedge)
	}

	if _, err := os.Stat(abs); os.IsNotExist(err) {
		if err := createTargetFile(dataDir, abs, parsed); err != nil {
			return err
		}
	}

	current, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	updated, err := writeTargetContent(string(current), parsed, comment, packet.Body)
	if err != nil {
		return err
	}
	return atomicWrite(abs, []byte(updated))
}

func createTargetFile(dataDir, abs string, parsed applyTarget) error {
	if parsed.family == "proper" && strings.HasPrefix(parsed.ownerKey, "proper/") {
		feastID := strings.TrimPrefix(parsed.ownerKey, "proper/")
		if _, err := scaffold.EnsurePropers(dataDir, scaffold.Options{Write: true, FeastID: feastID}); err != nil {
			// Unknown or unloadable feast: fall through to a one-section file.
			_ = err
		}
		if _, err := os.Stat(abs); err == nil {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return atomicWrite(abs, []byte{})
}

func writeTargetContent(current string, parsed applyTarget, comment, body string) (string, error) {
	body = strings.TrimRight(body, "\n")
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("refusing empty section body")
	}
	if parsed.slot == "" || parsed.family == "canticles" || parsed.ownerKey == "ordinary/marian" {
		return renderWholeFile(comment, body), nil
	}
	return replaceOrInsertSection(current, parsed.slot, comment, body)
}

// relativeTextPath maps a corpus key to a path under texts/.
func relativeTextPath(targetKey string) (string, error) {
	parsed, err := parseApplyTarget(targetKey)
	if err != nil {
		return "", err
	}
	if parsed.psalm {
		return "", fmt.Errorf("psalms are not writable")
	}
	switch parsed.family {
	case "canticles":
		return filepath.ToSlash(filepath.Join("texts", "canticles", parsed.slot+".txt")), nil
	case "shared":
		file := strings.TrimPrefix(parsed.ownerKey, "shared/")
		if file == "" || file == parsed.ownerKey {
			return "", fmt.Errorf("shared target has no file")
		}
		return filepath.ToSlash(filepath.Join("texts", "shared", file+".txt")), nil
	case "proper", "commons", "seasonal":
		owner := strings.TrimPrefix(parsed.ownerKey, parsed.family+"/")
		return filepath.ToSlash(filepath.Join("texts", parsed.family, owner+".txt")), nil
	case "ordinary":
		rest := strings.TrimPrefix(parsed.ownerKey, "ordinary/")
		if rest == "marian" {
			return filepath.ToSlash(filepath.Join("texts", "ordinary", "marian", parsed.slot+".txt")), nil
		}
		return filepath.ToSlash(filepath.Join("texts", "ordinary", rest+".txt")), nil
	default:
		return "", fmt.Errorf("unsupported target family %q", parsed.family)
	}
}

func confinedTextPath(dataDir, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return "", fmt.Errorf("text path %q is unsafe", rel)
	}
	root := filepath.Join(dataDir, "texts")
	abs := filepath.Join(dataDir, filepath.FromSlash(rel))
	relToRoot, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(relToRoot, "..") {
		return "", fmt.Errorf("text path escapes data/texts")
	}
	if relToRoot == "psalms" || strings.HasPrefix(relToRoot, "psalms"+string(filepath.Separator)) {
		return "", fmt.Errorf("psalms are not writable")
	}
	return abs, nil
}

func renderWholeFile(comment, body string) string {
	return "# SOURCE: " + comment + "\n" + body + "\n"
}

// replaceOrInsertSection rewrites one INI section and leaves every other
// live section untouched.
func replaceOrInsertSection(content, slot, comment, body string) (string, error) {
	if !ownerIDRE.MatchString(slot) {
		return "", fmt.Errorf("slot %q is not a safe section name", slot)
	}
	lines := splitKeepEnd(content)
	start, end, found := findSection(lines, slot)
	block := renderSection(slot, comment, body)
	if !found {
		out := strings.TrimRight(content, "\n")
		if out != "" {
			out += "\n\n"
		}
		return out + block, nil
	}
	var b strings.Builder
	for i := 0; i < start; i++ {
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	b.WriteString(block)
	if end < len(lines) {
		// Preserve a separating blank line before the next header when the
		// original region already ended at that header.
		if !strings.HasSuffix(block, "\n\n") {
			b.WriteByte('\n')
		}
		for i := end; i < len(lines); i++ {
			b.WriteString(lines[i])
			if i < len(lines)-1 || contentEndsWithNewline(content) {
				b.WriteByte('\n')
			}
		}
	}
	return b.String(), nil
}

func renderSection(slot, comment, body string) string {
	return "[" + slot + "]\n# SOURCE: " + comment + "\n" + body + "\n"
}

func findSection(lines []string, slot string) (start, end int, found bool) {
	start = -1
	for i, line := range lines {
		name, ok := sectionName(line)
		if !ok {
			continue
		}
		if name == slot && start < 0 {
			start = i
			continue
		}
		if start >= 0 && name != slot {
			return start, i, true
		}
	}
	if start >= 0 {
		return start, len(lines), true
	}
	return 0, 0, false
}

func sectionName(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if m := liveSectionLine.FindStringSubmatch(trimmed); m != nil {
		return m[1], true
	}
	if m := commentedSectionLine.FindStringSubmatch(trimmed); m != nil {
		return m[1], true
	}
	return "", false
}

func splitKeepEnd(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

func contentEndsWithNewline(content string) bool {
	return strings.HasSuffix(content, "\n")
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".apply-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
