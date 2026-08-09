package texts

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAssumptionWeekCommemorationRouting(t *testing.T) {
	corpus, err := LoadTexts(filepath.Join("..", "..", "data"))
	if err != nil {
		t.Fatalf("LoadTexts: %v", err)
	}

	for _, tt := range []struct {
		ref  string
		want string
	}{
		{"proper/st-eusebius-august/commemoration-antiphon-vespers", "I will liken him"},
		{"proper/st-helen/commemoration-antiphon-vespers", "The kingdom of heaven"},
		{"proper/st-helen/commemoration-antiphon-lauds", "Give her"},
		{"proper/st-agapitus/commemoration-antiphon-vespers", "This is a Martyr"},
		{"proper/st-agapitus/commemoration-antiphon-lauds", "The very hairs"},
	} {
		t.Run(tt.ref, func(t *testing.T) {
			if got := corpus.Get(tt.ref); !strings.HasPrefix(got, tt.want) {
				t.Errorf("%s = %q, want prefix %q", tt.ref, got, tt.want)
			}
		})
	}
}
