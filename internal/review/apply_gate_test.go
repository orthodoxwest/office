package review

import (
	"strings"
	"testing"
)

func validPacket() ApplyPacket {
	return ApplyPacket{
		CandidateID:    "DI-aaaaaaaaaaaa-P78-bbbbbbbbbbbb",
		SourceSHA256:   strings.Repeat("ab", 32),
		Extractor:      "pdftotext-layout",
		Pages:          []ApplyPage{{Page: 78, PrintedPage: "78", RawTextSHA256: strings.Repeat("cd", 32)}},
		TargetKey:      "proper/st-benedict/chapter-lauds",
		Action:         ApplyAddSection,
		Body:           "!Sir 50:6-7\nBehold a great Confessor.\nR. Thanks be to God.",
		DiscoveryClass: ClassMissingOverride,
		TextSimilarity: 0.27,
	}
}

func validWorld() ApplyWorld {
	return ApplyWorld{
		ExistingKeys: map[string]bool{
			"proper/st-benedict/collect": true,
			"commons/martyr/hymn-lauds":  true,
		},
		KnownOwners: map[string]bool{
			"proper/st-benedict": true,
			"commons/martyr":     true,
			"ordinary/lauds":     true,
			"canticles":          true,
			"shared":             true,
		},
		VerifiedKeys: map[string]bool{
			"commons/martyr/hymn-lauds": true,
		},
	}
}

func TestGateAllowsMissingOverrideAdd(t *testing.T) {
	d := Gate(validPacket(), validWorld())
	if d.Status != GateAllow {
		t.Fatalf("status = %s reasons = %v", d.Status, d.Reasons)
	}
	if d.UnseatsVerified {
		t.Fatal("add should not unseat a verified key")
	}
}

func TestGateAllowsTwoPageSpan(t *testing.T) {
	p := validPacket()
	p.Pages = []ApplyPage{
		{Page: 80, PrintedPage: "80", RawTextSHA256: strings.Repeat("11", 32)},
		{Page: 81, PrintedPage: "81", RawTextSHA256: strings.Repeat("22", 32)},
	}
	p.TargetKey = "proper/st-benedict/hymn-lauds"
	p.Body = "Laudibus cives resonent canoris\n\nGem of the highest, diadem immortal."
	d := Gate(p, validWorld())
	if d.Status != GateAllow {
		t.Fatalf("two-page span refused: %s %v", d.Status, d.Reasons)
	}
}

func TestGateNoopFallbackEqual(t *testing.T) {
	p := validPacket()
	p.DiscoveryClass = ClassFallbackEqual
	d := Gate(p, validWorld())
	if d.Status != GateNoop {
		t.Fatalf("status = %s reasons = %v", d.Status, d.Reasons)
	}
}

func TestGateNoopVerifyExisting(t *testing.T) {
	p := validPacket()
	p.DiscoveryClass = ClassVerifyExisting
	d := Gate(p, validWorld())
	if d.Status != GateNoop {
		t.Fatalf("status = %s", d.Status)
	}
}

func TestGateRefusesForbiddenClasses(t *testing.T) {
	for _, class := range []string{
		ClassAmbiguousOwner, ClassUnknownFeast, ClassUnmodeledSlot,
		ClassNeedsRuling, ClassRubricalComplex, ClassKnownUnobserved, "made-up",
	} {
		p := validPacket()
		p.DiscoveryClass = class
		d := Gate(p, validWorld())
		if d.Status != GateRefuse {
			t.Errorf("%s: status = %s", class, d.Status)
		}
	}
}

func TestGateRefusesPsalms(t *testing.T) {
	p := validPacket()
	p.TargetKey = "psalms/067"
	p.Action = ApplyReplaceSection
	world := validWorld()
	world.ExistingKeys["psalms/067"] = true
	world.KnownOwners["psalms"] = true
	d := Gate(p, world)
	if d.Status != GateRefuse || !hasReason(d, "psalms") {
		t.Fatalf("psalm not refused: %s %v", d.Status, d.Reasons)
	}
}

func TestGateFlagsVerifiedReplace(t *testing.T) {
	p := validPacket()
	p.TargetKey = "commons/martyr/hymn-lauds"
	p.Action = ApplyReplaceSection
	p.DiscoveryClass = ClassExistingDifferent
	p.TextSimilarity = 0.72
	p.Body = "Martyr of God, whose strength was from on high."
	d := Gate(p, validWorld())
	if d.Status != GateAllow {
		t.Fatalf("verified replace blocked: %s %v", d.Status, d.Reasons)
	}
	if !d.UnseatsVerified {
		t.Fatal("expected UnseatsVerified")
	}
}

func TestGateRefusesMixedExtractorRoutes(t *testing.T) {
	p := validPacket()
	p.Extractor = "pdftotext-layout"
	p.Pages = []ApplyPage{
		{Page: 80, Extractor: "pdftotext-layout", RawTextSHA256: strings.Repeat("11", 32)},
		{Page: 81, Extractor: "tesseract", RawTextSHA256: strings.Repeat("22", 32)},
	}
	d := Gate(p, validWorld())
	if d.Status != GateRefuse || !hasReason(d, "mixed extractor") {
		t.Fatalf("mixed routes: %s %v", d.Status, d.Reasons)
	}
}

func TestGateRefusesNonConsecutivePages(t *testing.T) {
	p := validPacket()
	p.Pages = []ApplyPage{
		{Page: 80, RawTextSHA256: strings.Repeat("11", 32)},
		{Page: 82, RawTextSHA256: strings.Repeat("22", 32)},
	}
	d := Gate(p, validWorld())
	if d.Status != GateRefuse || !hasReason(d, "consecutive") {
		t.Fatalf("gapped span: %s %v", d.Status, d.Reasons)
	}
}

func TestGateRefusesUnknownOwner(t *testing.T) {
	p := validPacket()
	p.TargetKey = "proper/st-nobody/collect"
	p.Body = "O God, who didst raise up thy servant."
	d := Gate(p, validWorld())
	if d.Status != GateRefuse || !hasReason(d, "not a known") {
		t.Fatalf("unknown owner: %s %v", d.Status, d.Reasons)
	}
}

func TestGateRefusesAddWhenKeyExists(t *testing.T) {
	p := validPacket()
	p.TargetKey = "proper/st-benedict/collect"
	p.Body = "Almighty and everlasting God."
	d := Gate(p, validWorld())
	if d.Status != GateRefuse || !hasReason(d, "already exists") {
		t.Fatalf("add existing: %s %v", d.Status, d.Reasons)
	}
}

func TestGateRefusesReplaceWhenKeyMissing(t *testing.T) {
	p := validPacket()
	p.Action = ApplyReplaceSection
	p.DiscoveryClass = ClassExistingDifferent
	p.TextSimilarity = 0.70
	d := Gate(p, validWorld())
	if d.Status != GateRefuse || !hasReason(d, "not an existing") {
		t.Fatalf("replace missing: %s %v", d.Status, d.Reasons)
	}
}

func TestGateRefusesLowReplaceSimilarity(t *testing.T) {
	p := validPacket()
	p.TargetKey = "proper/st-benedict/collect"
	p.Action = ApplyReplaceSection
	p.DiscoveryClass = ClassExistingDifferent
	p.TextSimilarity = 0.20
	p.Body = "Almighty and everlasting God."
	d := Gate(p, validWorld())
	if d.Status != GateRefuse || !hasReason(d, "below") {
		t.Fatalf("low similarity: %s %v", d.Status, d.Reasons)
	}
}

func TestGateRefusesAddThatLooksLikeFallback(t *testing.T) {
	p := validPacket()
	p.TextSimilarity = 0.97
	d := Gate(p, validWorld())
	if d.Status != GateRefuse || !hasReason(d, "fallback") {
		t.Fatalf("near-fallback add: %s %v", d.Status, d.Reasons)
	}
}

func TestGateRefusesEmptyBody(t *testing.T) {
	p := validPacket()
	p.Body = "   \n"
	d := Gate(p, validWorld())
	if d.Status != GateRefuse || !hasReason(d, "empty") {
		t.Fatalf("empty body: %s %v", d.Status, d.Reasons)
	}
}

func TestGateRefusesVersicleWithoutResponses(t *testing.T) {
	p := validPacket()
	p.TargetKey = "proper/st-benedict/versicle-lauds"
	p.Body = "They declared the work of God."
	d := Gate(p, validWorld())
	if d.Status != GateRefuse || !hasReason(d, "V. and R.") {
		t.Fatalf("versicle: %s %v", d.Status, d.Reasons)
	}
}

func TestGateAllowsVersicleWithSigils(t *testing.T) {
	p := validPacket()
	p.TargetKey = "proper/st-benedict/versicle-lauds"
	p.Body = "V. They declared the work of God.\nR. And wisely considered of his doing."
	d := Gate(p, validWorld())
	if d.Status != GateAllow {
		t.Fatalf("versicle refused: %s %v", d.Status, d.Reasons)
	}
}

func TestGateRefusesCollectUnderlay(t *testing.T) {
	p := validPacket()
	p.TargetKey = "proper/st-benedict/collect"
	p.Body = "Al-migh-ty and ev-er-last-ing God who didst."
	d := Gate(p, validWorld())
	if d.Status != GateRefuse || !hasReason(d, "chant underlay") {
		t.Fatalf("collect underlay: %s %v", d.Status, d.Reasons)
	}
}

func TestGateRefusesPsalmFamilyAndPathEscape(t *testing.T) {
	p := validPacket()
	p.TargetKey = "proper/../feasts/secret"
	d := Gate(p, validWorld())
	if d.Status != GateRefuse {
		t.Fatalf("path escape allowed: %v", d.Reasons)
	}
}

func TestGateRefusesSpanOverCap(t *testing.T) {
	p := validPacket()
	p.Pages = []ApplyPage{
		{Page: 1, RawTextSHA256: strings.Repeat("11", 32)},
		{Page: 2, RawTextSHA256: strings.Repeat("22", 32)},
		{Page: 3, RawTextSHA256: strings.Repeat("33", 32)},
		{Page: 4, RawTextSHA256: strings.Repeat("44", 32)},
	}
	d := Gate(p, validWorld())
	if d.Status != GateRefuse || !hasReason(d, "exceeds cap") {
		t.Fatalf("over-long span: %s %v", d.Status, d.Reasons)
	}
}

func TestGateAllowsLongCanticleSpan(t *testing.T) {
	p := validPacket()
	p.TargetKey = "canticles/benedicite"
	p.Action = ApplyReplaceSection
	p.DiscoveryClass = ClassExistingDifferent
	p.TextSimilarity = 0.90
	p.Body = "O all ye Works of the Lord, bless ye the Lord."
	world := validWorld()
	world.ExistingKeys["canticles/benedicite"] = true
	pages := make([]ApplyPage, 8)
	for i := range pages {
		pages[i] = ApplyPage{Page: i + 1, RawTextSHA256: strings.Repeat("aa", 32)}
	}
	p.Pages = pages
	d := Gate(p, world)
	if d.Status != GateAllow {
		t.Fatalf("canticle span refused: %s %v", d.Status, d.Reasons)
	}
}

func TestFormatSourceCommentPageRange(t *testing.T) {
	got := FormatSourceComment("monastic-diurnal", []ApplyPage{
		{Page: 78, PrintedPage: "78"},
		{Page: 80, PrintedPage: "80"},
	}, "DI-abc", "chapter-lauds")
	if !strings.Contains(got, "p. 78-80") || !strings.Contains(got, SourceHedge) {
		t.Fatalf("comment = %q", got)
	}
}

func hasReason(d GateDecision, needle string) bool {
	for _, r := range d.Reasons {
		if strings.Contains(r, needle) {
			return true
		}
	}
	return false
}
