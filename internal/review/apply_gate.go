package review

import (
	"crypto/sha256"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/orthodoxwest/office/internal/scaffold"
)

// Apply actions a gated packet may request.
const (
	ApplyAddSection     = "add-section"
	ApplyReplaceSection = "replace-section"
)

// Discovery classes the gate will write for, treat as no-ops, or refuse.
const (
	ClassMissingOverride   = "missing-override"
	ClassExistingDifferent = "existing-different"
	ClassFallbackEqual     = "fallback-equal"
	ClassVerifyExisting    = "verify-existing"
	ClassAmbiguousOwner    = "ambiguous-owner/context"
	ClassUnknownFeast      = "unknown-feast"
	ClassUnmodeledSlot     = "unmodeled-slot"
	ClassNeedsRuling       = "needs-ruling"
	ClassRubricalComplex   = "rubrical-complex"
	ClassKnownUnobserved   = "known-owner-unobserved"
)

// Similarity bounds: replace must look like the same slot; add must not
// look like the runtime fallback in disguise.
const (
	ReplaceMinSimilarity     = 0.55
	AddMaxFallbackSimilarity = 0.84
)

// Continuation span caps. A missed heading must not swallow the next office.
const (
	MaxSpanPagesDefault  = 3
	MaxSpanPagesCanticle = 12
)

// GateStatus is the mechanical decision for one apply packet.
type GateStatus string

const (
	GateAllow  GateStatus = "allow"
	GateRefuse GateStatus = "refuse"
	GateNoop   GateStatus = "noop"
)

// ApplyPage is one hashed page slice in a (possibly continued) witness.
type ApplyPage struct {
	Page          int       `json:"page"`
	PrintedPage   string    `json:"printed_page,omitempty"`
	RawTextSHA256 string    `json:"raw_text_sha256"`
	SourceColumn  string    `json:"source_column,omitempty"`
	BBox          []float64 `json:"bbox,omitempty"`
	Offset        string    `json:"offset,omitempty"`
	Extractor     string    `json:"extractor,omitempty"`
}

// ApplyPacket is the only input the writer may honor. Model self-score is
// not a field.
type ApplyPacket struct {
	CandidateID    string      `json:"candidate_id"`
	SourceSHA256   string      `json:"source_sha256"`
	RawTextSHA256  string      `json:"raw_text_sha256"`
	Extractor      string      `json:"extractor"`
	Pages          []ApplyPage `json:"pages"`
	TargetKey      string      `json:"target_key"`
	Action         string      `json:"action"`
	Body           string      `json:"body"`
	SourceComment  string      `json:"source_comment,omitempty"`
	DiscoveryClass string      `json:"discovery_class"`
	TextSimilarity float64     `json:"text_similarity"`
}

// ApplyWorld is the corpus/calendar snapshot the gate consults. Tests
// supply it directly; the writer will load it from data/.
type ApplyWorld struct {
	ExistingKeys map[string]bool
	KnownOwners  map[string]bool
	VerifiedKeys map[string]bool
}

// GateDecision is the mechanical verdict. UnseatsVerified is set when an
// allowed replace would stale a current attestation; the packet is still
// allowed, but those PRs must not mix with routine adds.
type GateDecision struct {
	Status          GateStatus
	Reasons         []string
	UnseatsVerified bool
}

var (
	ownerIDRE        = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	columnIDRE       = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	sha256RE         = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
	underlayRE       = regexp.MustCompile(`[A-Za-z][-–][A-Za-z]`)
	puaRE            = regexp.MustCompile(`\x{FFFD}|[\x{E000}-\x{F8FF}]`)
	crossRefRE       = regexp.MustCompile(`(?is)(?:from|see|as in)\s+(?:the\s+)?(?:lauds|vespers|matins|compline)\b.*\bcommon\b.*(?:/|$)`)
	prayersHeadingRE = regexp.MustCompile(`(?im)^[ \t]*the prayers[ \t]*$`)
)

var allowedClasses = map[string]bool{
	ClassMissingOverride:   true,
	ClassExistingDifferent: true,
}

var noopClasses = map[string]bool{
	ClassFallbackEqual:  true,
	ClassVerifyExisting: true,
}

var forbiddenClasses = map[string]bool{
	ClassAmbiguousOwner:  true,
	ClassUnknownFeast:    true,
	ClassUnmodeledSlot:   true,
	ClassNeedsRuling:     true,
	ClassRubricalComplex: true,
	ClassKnownUnobserved: true,
}

var properSlots = func() map[string]bool {
	out := make(map[string]bool)
	for _, k := range scaffold.AllKeys() {
		out[k.Key] = true
	}
	// Bare hour-unqualified spellings the engine also resolves.
	for _, k := range []string{"chapter", "hymn", "versicle", "short-responsory"} {
		out[k] = true
	}
	return out
}()

// Gate returns the mechanical decision for one packet. It never writes
// files and never consults a model score.
func Gate(packet ApplyPacket, world ApplyWorld) GateDecision {
	d := GateDecision{Status: GateAllow}
	refuse := func(reason string) {
		d.Status = GateRefuse
		d.Reasons = append(d.Reasons, reason)
	}
	note := func(reason string) {
		d.Reasons = append(d.Reasons, reason)
	}

	if packet.CandidateID == "" {
		refuse("candidate_id is required")
	}
	if !sha256RE.MatchString(packet.SourceSHA256) {
		refuse("source_sha256 must be a full SHA-256")
	}
	if !sha256RE.MatchString(packet.RawTextSHA256) {
		refuse("raw_text_sha256 must be a full SHA-256")
	}
	if strings.TrimSpace(packet.Extractor) == "" {
		refuse("extractor is required")
	}

	class := packet.DiscoveryClass
	switch {
	case noopClasses[class]:
		d.Status = GateNoop
		note("discovery class " + class + " is a no-op")
		return d
	case allowedClasses[class]:
		// continue
	case forbiddenClasses[class] || class == "":
		if class == "" {
			refuse("discovery class is required")
		} else {
			refuse("discovery class " + class + " is not writable")
		}
	default:
		refuse("discovery class " + class + " is not writable")
	}

	switch packet.Action {
	case ApplyAddSection, ApplyReplaceSection:
	default:
		refuse("action must be add-section or replace-section")
	}

	parsed, err := parseApplyTarget(packet.TargetKey)
	if err != nil {
		refuse(err.Error())
	} else {
		if parsed.psalm {
			refuse("psalms are out of scope")
		}
		ownerKey := parsed.ownerKey
		if ownerKey != "" && !world.KnownOwners[ownerKey] {
			refuse("owner " + ownerKey + " is not a known calendar or corpus owner")
		}
		if parsed.family == "proper" && parsed.slot != "" && !properSlots[parsed.slot] {
			refuse("slot " + parsed.slot + " is not a catalog proper section")
		}
		exists := world.ExistingKeys[packet.TargetKey]
		switch packet.Action {
		case ApplyAddSection:
			if exists {
				refuse("add-section target already exists; use replace-section")
			}
		case ApplyReplaceSection:
			if !exists {
				refuse("replace-section target is not an existing corpus key")
			}
		}
		if world.VerifiedKeys[packet.TargetKey] && packet.Action == ApplyReplaceSection {
			d.UnseatsVerified = true
			note("replace unseats a verified attestation")
		}
	}

	checkSpan(&d, packet, parsed)

	body := packet.Body
	if strings.TrimSpace(body) == "" {
		refuse("body is empty")
	} else {
		if puaRE.MatchString(body) {
			refuse("body contains replacement or private-use glyphs")
		}
		if reasons := genreReasons(packet.TargetKey, body); len(reasons) > 0 {
			for _, r := range reasons {
				refuse(r)
			}
		}
		if crossRefRE.MatchString(body) {
			refuse("body contains a printed cross-reference or directive fragment")
		}
	}

	switch packet.Action {
	case ApplyReplaceSection:
		if packet.TextSimilarity < ReplaceMinSimilarity {
			refuse(fmt.Sprintf("replace similarity %.3f is below %.2f; slot identity is unproven", packet.TextSimilarity, ReplaceMinSimilarity))
		}
	case ApplyAddSection:
		if packet.TextSimilarity >= AddMaxFallbackSimilarity {
			refuse(fmt.Sprintf("add similarity %.3f is at or above %.2f; printed text looks like the runtime fallback", packet.TextSimilarity, AddMaxFallbackSimilarity))
		}
	}

	if d.Status == GateRefuse {
		return d
	}
	if len(d.Reasons) == 0 {
		d.Reasons = []string{"passed mechanical gate"}
	}
	return d
}

type applyTarget struct {
	family   string
	ownerKey string
	slot     string
	psalm    bool
}

func parseApplyTarget(key string) (applyTarget, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return applyTarget{}, fmt.Errorf("target_key is required")
	}
	if strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return applyTarget{}, fmt.Errorf("target_key is not a safe corpus path")
	}
	parts := strings.Split(key, "/")
	if len(parts) < 2 {
		return applyTarget{}, fmt.Errorf("target_key %q is not a corpus key", key)
	}
	for _, p := range parts {
		if p == "" || !ownerIDRE.MatchString(p) {
			return applyTarget{}, fmt.Errorf("target_key %q has an invalid path segment", key)
		}
	}
	t := applyTarget{family: parts[0]}
	switch parts[0] {
	case "psalms":
		t.psalm = true
		t.ownerKey = "psalms"
		t.slot = strings.Join(parts[1:], "/")
	case "proper", "commons", "seasonal":
		if len(parts) < 3 {
			return applyTarget{}, fmt.Errorf("target_key %q needs owner and slot", key)
		}
		t.ownerKey = parts[0] + "/" + parts[1]
		t.slot = strings.Join(parts[2:], "/")
	case "ordinary":
		if len(parts) < 3 {
			return applyTarget{}, fmt.Errorf("target_key %q needs hour and slot", key)
		}
		t.ownerKey = parts[0] + "/" + parts[1]
		t.slot = strings.Join(parts[2:], "/")
	case "canticles":
		t.ownerKey = "canticles"
		t.slot = strings.Join(parts[1:], "/")
	case "shared":
		if len(parts) < 3 {
			return applyTarget{}, fmt.Errorf("target_key %q needs file and slot", key)
		}
		t.ownerKey = parts[0] + "/" + parts[1]
		t.slot = strings.Join(parts[2:], "/")
	default:
		return applyTarget{}, fmt.Errorf("target_key family %q is not writable", parts[0])
	}
	return t, nil
}

func checkSpan(d *GateDecision, packet ApplyPacket, parsed applyTarget) {
	refuse := func(reason string) {
		d.Status = GateRefuse
		d.Reasons = append(d.Reasons, reason)
	}
	if len(packet.Pages) == 0 {
		refuse("pages must list at least one witness slice")
		return
	}
	maxPages := MaxSpanPagesDefault
	if parsed.family == "canticles" {
		maxPages = MaxSpanPagesCanticle
	}
	if len(packet.Pages) > maxPages {
		refuse(fmt.Sprintf("span of %d pages exceeds cap %d", len(packet.Pages), maxPages))
	}
	route := strings.TrimSpace(packet.Extractor)
	prev := 0
	column := ""
	sawPlain := false
	var sliceHashes []string
	for i, page := range packet.Pages {
		if page.Page < 1 {
			refuse(fmt.Sprintf("pages[%d] needs a positive page number", i))
			continue
		}
		if !sha256RE.MatchString(page.RawTextSHA256) {
			refuse(fmt.Sprintf("pages[%d] raw_text_sha256 must be a full SHA-256", i))
		} else {
			sliceHashes = append(sliceHashes, strings.ToLower(page.RawTextSHA256))
		}
		pageRoute := strings.TrimSpace(page.Extractor)
		if page.SourceColumn != "" {
			if sawPlain {
				refuse("mixed column and non-column pages cannot form one witness")
			}
			if !columnIDRE.MatchString(page.SourceColumn) {
				refuse(fmt.Sprintf("pages[%d] source_column is unsafe", i))
			}
			if column == "" {
				column = page.SourceColumn
			} else if column != page.SourceColumn {
				refuse("mixed source columns cannot form one witness")
			}
			if len(page.BBox) != 4 || page.BBox[2] <= page.BBox[0] || page.BBox[3] <= page.BBox[1] {
				refuse(fmt.Sprintf("pages[%d] bbox must be four positive finite coordinates", i))
			} else {
				for _, value := range page.BBox {
					if math.IsNaN(value) || math.IsInf(value, 0) {
						refuse(fmt.Sprintf("pages[%d] bbox must be finite", i))
						break
					}
				}
			}
		} else if column != "" {
			refuse("mixed column and non-column pages cannot form one witness")
		} else {
			sawPlain = true
		}
		if pageRoute != "" && route != "" && pageRoute != route {
			refuse("mixed extractor routes cannot form one witness")
		}
		if i > 0 && page.Page != prev+1 {
			refuse("pages must be consecutive in one document")
		}
		prev = page.Page
	}
	if len(sliceHashes) == len(packet.Pages) && strings.ToLower(packet.RawTextSHA256) != compositeSliceHashes(sliceHashes) {
		refuse("raw_text_sha256 disagrees with ordered page slice hashes")
	}
}

func compositeSliceHashes(hashes []string) string {
	if len(hashes) == 1 {
		return strings.ToLower(hashes[0])
	}
	sum := sha256.Sum256([]byte(strings.Join(hashes, "\x1f")))
	return fmt.Sprintf("%x", sum)
}

func genreReasons(key, body string) []string {
	var reasons []string
	slot := key
	if i := strings.LastIndex(key, "/"); i >= 0 {
		slot = key[i+1:]
	}
	if strings.Contains(slot, "versicle") || strings.Contains(slot, "short-responsory") {
		if !hasSigil(body, "V.") || !hasSigil(body, "R.") {
			reasons = append(reasons, "versicle or responsory body must contain V. and R.")
		}
	}
	if chantUnderlayCount(body) >= 4 {
		reasons = append(reasons, "body looks like chant underlay, not corpus text")
	}
	lowered := strings.ToLower(body)
	for _, leftover := range []string{"benedictus, tone", "after the canticle", "\ncollect\n", "the gospel canticle"} {
		if strings.Contains(lowered, leftover) {
			reasons = append(reasons, "body contains leftover rubric or the next office slot")
			break
		}
	}
	if prayersHeadingRE.MatchString(body) {
		reasons = append(reasons, "body contains leftover rubric or the next office slot")
	}
	if hasControlBesidesNewline(body) {
		reasons = append(reasons, "body contains a control character")
	}
	return reasons
}

func hasSigil(body, sigil string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimLeftFunc(line, unicode.IsSpace), sigil) {
			return true
		}
	}
	return strings.Contains(body, sigil)
}

func chantUnderlayCount(body string) int {
	return len(underlayRE.FindAllString(body, -1))
}

func hasControlBesidesNewline(body string) bool {
	for _, r := range body {
		if r < 0x20 && r != '\n' && r != '\t' {
			return true
		}
	}
	return false
}

// SourceHedge is the required phrase in an agent-proposed SOURCE comment.
const SourceHedge = "agent-proposed, not attested"

// FormatSourceComment builds the citation written beside an applied section.
func FormatSourceComment(source string, pages []ApplyPage, candidateID, slot string) string {
	printed := pageRange(pages)
	parts := []string{strings.TrimSpace(source)}
	if printed != "" {
		parts = append(parts, "p. "+printed)
	}
	detail := candidateID
	if slot != "" {
		detail = candidateID + "; " + slot
	}
	if detail != "" {
		parts = append(parts, "("+detail+")")
	}
	return strings.TrimSpace(strings.Join(parts, " ")) + " — " + SourceHedge
}

func pageRange(pages []ApplyPage) string {
	if len(pages) == 0 {
		return ""
	}
	first := pages[0].PrintedPage
	if first == "" {
		first = fmt.Sprintf("%d", pages[0].Page)
	}
	if len(pages) == 1 {
		return first
	}
	last := pages[len(pages)-1].PrintedPage
	if last == "" {
		last = fmt.Sprintf("%d", pages[len(pages)-1].Page)
	}
	if first == last {
		return first
	}
	return first + "-" + last
}
