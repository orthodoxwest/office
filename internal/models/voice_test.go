package models

import (
	"reflect"
	"testing"
)

func TestSpeakerTurnsRequireCompleteSpokenPrayer(t *testing.T) {
	prayer := OfficeElement{Text: "Mercy.\nAmen.", Voice: []VoiceSpan{
		{Text: "Mercy.\n", Spoken: true, Role: VoicePriest},
		{Text: "Amen.", Spoken: true, Role: VoiceResponse},
	}}
	if turns := prayer.SpeakerTurns(); !reflect.DeepEqual(turns, prayer.Voice) || turns[0].Role.Label() != "Priest" || turns[1].Role.Label() != "People" {
		t.Fatalf("lost prayer turns or speaker names: %v", turns)
	}
	for _, role := range []VoiceRole{VoiceOfficiant, VoiceAll} {
		elem := OfficeElement{Text: "Amen.", Voice: []VoiceSpan{{Text: "Amen.", Spoken: true, Role: role}}}
		if len(elem.SpeakerTurns()) != 1 || role.Label() == "" {
			t.Errorf("missing shared speaker %q", role)
		}
	}
	for _, tc := range []struct {
		name  string
		voice []VoiceSpan
	}{
		{"missing", nil},
		{"truncated", prayer.Voice[:1]},
		{"extra", append(append([]VoiceSpan{}, prayer.Voice...), VoiceSpan{Text: "Extra.", Spoken: true, Role: VoiceAll})},
		{"silent", []VoiceSpan{{Text: prayer.Text, Role: VoiceAll}}},
		{"unspecified speaker", []VoiceSpan{{Text: prayer.Text, Spoken: true}}},
		{"unknown speaker", []VoiceSpan{{Text: prayer.Text, Spoken: true, Role: "unknown"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			elem := OfficeElement{Text: prayer.Text, Voice: tc.voice}
			if turns := elem.SpeakerTurns(); turns != nil {
				t.Fatalf("accepted invalid speaker partition: %v", turns)
			}
		})
	}
}
