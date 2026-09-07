package render

import (
	"fmt"
	"strings"
	"testing"

	"github.com/orthodoxwest/office/internal/models"
	"github.com/orthodoxwest/office/internal/texts"
)

func TestRenderAthanasianCreedCorpus(t *testing.T) {
	corpus, err := texts.LoadTexts("../../data")
	if err != nil {
		t.Fatal(err)
	}
	html := string(renderSectionElements([]models.OfficeElement{
		{Type: models.Canticle, Label: "Athanasian Creed", Text: corpus.Get("proper/trinity-sunday/athanasian-creed")},
		{Type: models.PsalmDoxology, Text: corpus.Get("ordinary/shared/gloria-patri")},
	}))
	for n := 1; n <= 42; n++ {
		if !strings.Contains(html, fmt.Sprintf(`<span class="verse-num">%d</span>`, n)) {
			t.Errorf("verse %d is missing from the creed HTML", n)
		}
	}
	for _, want := range []string{"Whosoever will be saved", "The Holy Ghost is of the Father through the Son:", "which except a man believe faithfully, he cannot be saved.", "Glory be to the Father"} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered creed is missing %q", want)
		}
	}
}
