package office

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestBuildPrayerVoiceSecret(t *testing.T) {
	text := "Our Father, who art in heaven, Hallowed be thy Name.\nThy kingdom come."
	got := buildPrayerVoice("ordinary/shared/our-father", text, false)
	want := []models.VoiceSpan{
		{Text: "Our Father", Spoken: true},
		{Text: ", who art in heaven, Hallowed be thy Name.\nThy kingdom come.", Spoken: false},
	}
	assertVoice(t, got, want)
}

func TestBuildPrayerVoiceHailMaryAndCreed(t *testing.T) {
	hm := "Hail, Mary, full of grace, the Lord is with thee.\nAmen."
	got := buildPrayerVoice("ordinary/shared/hail-mary", hm, false)
	assertVoice(t, got, []models.VoiceSpan{
		{Text: "Hail, Mary", Spoken: true},
		{Text: ", full of grace, the Lord is with thee.\nAmen.", Spoken: false},
	})

	creed := "I believe in God the Father Almighty.\n\nAnd in Jesus Christ."
	got = buildPrayerVoice("ordinary/shared/apostles-creed", creed, false)
	assertVoice(t, got, []models.VoiceSpan{
		{Text: "I believe", Spoken: true},
		{Text: " in God the Father Almighty.\n\nAnd in Jesus Christ.", Spoken: false},
	})
}

func TestBuildPrayerVoicePartlySecret(t *testing.T) {
	text := strings.TrimSuffix(
		"Our Father, who art in heaven, Hallowed be thy Name.\n"+
			"And forgive us our trespasses, As we forgive those who trespass against us.\n"+
			"And lead us not into temptation,\n"+
			"But deliver us from evil. Amen.",
		" Amen.",
	)
	got := buildPrayerVoice("ordinary/shared/our-father", text, true)
	want := []models.VoiceSpan{
		{Text: "Our Father", Spoken: true},
		{Text: ", who art in heaven, Hallowed be thy Name.\n" +
			"And forgive us our trespasses, As we forgive those who trespass against us.\n", Spoken: false},
		{Text: "And lead us not into temptation,\nBut deliver us from evil.", Spoken: true},
	}
	assertVoice(t, got, want)

	// Concatenation must equal source text.
	var b strings.Builder
	for _, sp := range got {
		b.WriteString(sp.Text)
	}
	if b.String() != text {
		t.Fatalf("voice spans do not rejoin to text\n got %q\nwant %q", b.String(), text)
	}
}

func TestBuildPrayerVoiceUnknownRef(t *testing.T) {
	if got := buildPrayerVoice("ordinary/shared/kyrie", "Kyrie eleison.", false); got != nil {
		t.Fatalf("expected nil for unknown ref, got %+v", got)
	}
}

func assertVoice(t *testing.T, got, want []models.VoiceSpan) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d\n got %+v\nwant %+v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
