package scaffold

// Key describes one proper section the engine can resolve, for use in
// commented scaffolds. The Key field is the section name as it appears in a
// proper file (e.g. "collect", "chapter-lauds").
type Key struct {
	Key   string // section name without brackets
	Blurb string // one-line explanation shown under the commented header
	Tier  string // "core" or "optional"
}

// CoreKeys are the fill-in slots a normal sanctoral or temporal feast needs.
// They cover make audit's properRefs plus the usual gospel-canticle and
// indexed antiphon spellings.
var CoreKeys = []Key{
	{Key: "collect", Blurb: "Collect of the feast (all hours when celebrated). Use N. for the proper name if needed.", Tier: "core"},
	{Key: "benedictus-antiphon", Blurb: "Antiphon on the Benedictus at Lauds.", Tier: "core"},
	{Key: "magnificat-antiphon", Blurb: "Antiphon on the Magnificat at II Vespers.", Tier: "core"},
	{Key: "magnificat-antiphon-first", Blurb: "Antiphon on the Magnificat at I Vespers (omit if the same as II Vespers).", Tier: "core"},
	{Key: "psalm-antiphon", Blurb: "Single antiphon for all five psalms (fallback when psalm-antiphon-N is absent).", Tier: "core"},
	{Key: "psalm-antiphon-1", Blurb: "First psalm antiphon at Lauds (and Vespers unless a *-vespers override exists).", Tier: "core"},
	{Key: "psalm-antiphon-2", Blurb: "Second psalm antiphon at Lauds.", Tier: "core"},
	{Key: "psalm-antiphon-3", Blurb: "Third psalm antiphon at Lauds.", Tier: "core"},
	{Key: "psalm-antiphon-4", Blurb: "Fourth psalm antiphon at Lauds (often the canticle antiphon).", Tier: "core"},
	{Key: "psalm-antiphon-5", Blurb: "Fifth psalm antiphon at Lauds (Laudate psalms).", Tier: "core"},
	{Key: "commemoration-antiphon", Blurb: "Antiphon when this feast is only commemorated (not the day's celebration).", Tier: "core"},
	{Key: "commemoration-versicle", Blurb: "Versicle pair (V. … / R. …) for the commemoration of this feast.", Tier: "core"},
	{Key: "commemoration-collect", Blurb: "Collect for the commemoration (defaults to [collect] when omitted).", Tier: "core"},
}

// OptionalKeys are common hour-qualified spellings. Bare base slots (chapter,
// hymn, …) also resolve; these preferred forms match existing proper files.
var OptionalKeys = []Key{
	{Key: "chapter-first-vespers", Blurb: "Chapter at I Vespers when it differs from II Vespers.", Tier: "optional"},
	{Key: "chapter-lauds", Blurb: "Chapter (capitulum) at Lauds. Bare [chapter] also works for all hours.", Tier: "optional"},
	{Key: "chapter-prime", Blurb: "Proper chapter at Prime.", Tier: "optional"},
	{Key: "chapter-ferial-prime", Blurb: "Proper ferial chapter at Prime when the office explicitly supplies one.", Tier: "optional"},
	{Key: "chapter-terce", Blurb: "Proper chapter at Terce.", Tier: "optional"},
	{Key: "chapter-sext", Blurb: "Proper chapter at Sext.", Tier: "optional"},
	{Key: "chapter-none", Blurb: "Proper chapter at None.", Tier: "optional"},
	{Key: "chapter-vespers", Blurb: "Chapter at Vespers (often @use of chapter-lauds).", Tier: "optional"},
	{Key: "chapter-compline", Blurb: "Proper chapter at Compline.", Tier: "optional"},
	{Key: "hymn-first-vespers", Blurb: "Hymn at I Vespers when it differs from II Vespers.", Tier: "optional"},
	{Key: "hymn-lauds", Blurb: "Hymn at Lauds. First line may be a Latin incipit title.", Tier: "optional"},
	{Key: "hymn-prime", Blurb: "Proper hymn at Prime.", Tier: "optional"},
	{Key: "hymn-terce", Blurb: "Proper hymn at Terce.", Tier: "optional"},
	{Key: "hymn-sext", Blurb: "Proper hymn at Sext.", Tier: "optional"},
	{Key: "hymn-none", Blurb: "Proper hymn at None.", Tier: "optional"},
	{Key: "hymn-vespers", Blurb: "Hymn at Vespers.", Tier: "optional"},
	{Key: "hymn-compline", Blurb: "Proper hymn at Compline.", Tier: "optional"},
	{Key: "versicle-lauds", Blurb: "Versicle after the hymn at Lauds (V. … / R. …).", Tier: "optional"},
	{Key: "versicle-prime", Blurb: "Proper versicle at Prime.", Tier: "optional"},
	{Key: "versicle-terce", Blurb: "Proper versicle at Terce.", Tier: "optional"},
	{Key: "versicle-sext", Blurb: "Proper versicle at Sext.", Tier: "optional"},
	{Key: "versicle-none", Blurb: "Proper versicle at None.", Tier: "optional"},
	{Key: "versicle-vespers", Blurb: "Versicle after the hymn at Vespers.", Tier: "optional"},
	{Key: "versicle-first-vespers", Blurb: "Versicle at I Vespers when it differs from II Vespers.", Tier: "optional"},
	{Key: "versicle-compline", Blurb: "Proper versicle at Compline.", Tier: "optional"},
	{Key: "short-responsory-lauds", Blurb: "Short responsory at Lauds.", Tier: "optional"},
	{Key: "short-responsory-prime", Blurb: "Proper short responsory at Prime.", Tier: "optional"},
	{Key: "short-responsory-terce", Blurb: "Proper short responsory at Terce.", Tier: "optional"},
	{Key: "short-responsory-sext", Blurb: "Proper short responsory at Sext.", Tier: "optional"},
	{Key: "short-responsory-none", Blurb: "Proper short responsory at None.", Tier: "optional"},
	{Key: "short-responsory-vespers", Blurb: "Short responsory at Vespers.", Tier: "optional"},
	{Key: "short-responsory-first-vespers", Blurb: "Short responsory at I Vespers when it differs from II Vespers.", Tier: "optional"},
	{Key: "short-responsory-compline", Blurb: "Proper short responsory at Compline.", Tier: "optional"},
	{Key: "vespers-psalmody", Blurb: "Festal Vespers psalmody table (@use ordinary/vespers/festal-psalmody or a common).", Tier: "optional"},
	{Key: "vespers-psalmody-first", Blurb: "I Vespers psalmody table when it differs from II Vespers.", Tier: "optional"},
	{Key: "psalm-antiphon-1-vespers", Blurb: "Vespers-only override for psalm antiphon 1.", Tier: "optional"},
	{Key: "psalm-antiphon-2-vespers", Blurb: "Vespers-only override for psalm antiphon 2.", Tier: "optional"},
	{Key: "psalm-antiphon-3-vespers", Blurb: "Vespers-only override for psalm antiphon 3.", Tier: "optional"},
	{Key: "psalm-antiphon-4-vespers", Blurb: "Vespers-only override for psalm antiphon 4 (often @use of psalm-antiphon-5).", Tier: "optional"},
	{Key: "psalm-antiphon-5-vespers", Blurb: "Vespers-only override for psalm antiphon 5.", Tier: "optional"},
	{Key: "psalm-antiphon-1-first-vespers", Blurb: "I Vespers override for psalm antiphon 1.", Tier: "optional"},
	{Key: "psalm-antiphon-2-first-vespers", Blurb: "I Vespers override for psalm antiphon 2.", Tier: "optional"},
	{Key: "psalm-antiphon-3-first-vespers", Blurb: "I Vespers override for psalm antiphon 3.", Tier: "optional"},
	{Key: "psalm-antiphon-4-first-vespers", Blurb: "I Vespers override for psalm antiphon 4.", Tier: "optional"},
	{Key: "psalm-antiphon-5-first-vespers", Blurb: "I Vespers override for psalm antiphon 5.", Tier: "optional"},
}

// AllKeys returns core then optional keys in scaffold order.
func AllKeys() []Key {
	out := make([]Key, 0, len(CoreKeys)+len(OptionalKeys))
	out = append(out, CoreKeys...)
	out = append(out, OptionalKeys...)
	return out
}

// CommemorationKeys is the thinner set used when scaffolding rank=commemoration
// feasts (they rarely own a full proper office).
var CommemorationKeys = []Key{
	{Key: "commemoration-antiphon", Blurb: "Antiphon when this feast is commemorated.", Tier: "core"},
	{Key: "commemoration-antiphon-lauds", Blurb: "Benedictus antiphon when this feast is commemorated at Lauds.", Tier: "core"},
	{Key: "commemoration-antiphon-vespers", Blurb: "Magnificat antiphon when this feast is commemorated at Vespers.", Tier: "core"},
	{Key: "commemoration-versicle", Blurb: "Versicle pair (V. … / R. …) for the commemoration.", Tier: "core"},
	{Key: "commemoration-versicle-lauds", Blurb: "Versicle pair for the commemoration at Lauds.", Tier: "core"},
	{Key: "commemoration-versicle-vespers", Blurb: "Versicle pair for the commemoration at Vespers.", Tier: "core"},
	{Key: "commemoration-collect", Blurb: "Collect for the commemoration (defaults to a proper [collect] or the common).", Tier: "core"},
	{Key: "collect", Blurb: "Collect of this feast (used when it is celebrated, or as commemoration fallback).", Tier: "core"},
}

// KeysFor returns the catalog appropriate to a feast's rank.
func KeysFor(isCommemoration bool) []Key {
	if isCommemoration {
		return CommemorationKeys
	}
	return AllKeys()
}
