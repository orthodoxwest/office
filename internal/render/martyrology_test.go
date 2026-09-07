package render

import (
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
)

func TestMartyrologyReadingRendersParagraphsWithoutChapterHeading(t *testing.T) {
	got := renderOfficeElement(models.OfficeElement{
		Type: models.Reading,
		Text: "First notice.\n\nSecond notice.",
	}, "")
	if strings.Contains(got, "Chapter") || strings.Count(got, "<p") != 2 {
		t.Fatalf("reading must contain two prose paragraphs without a chapter heading: %s", got)
	}
	for _, want := range []string{"First notice.", "Second notice."} {
		if !strings.Contains(got, want) {
			t.Fatalf("reading missing %q: %s", want, got)
		}
	}
	response := renderOfficeElement(models.OfficeElement{Type: models.Response, Text: "R. Thanks be to God."}, "")
	if !strings.Contains(response, "℟.") || !strings.Contains(response, "Thanks be to God.") {
		t.Fatalf("missing response sigil: %s", response)
	}
}
