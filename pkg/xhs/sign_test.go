package xhs

import (
	"strings"
	"testing"
)

func TestSearchFeedsJS(t *testing.T) {
	// Arrange + Act
	js := searchFeedsJS()

	// Assert: reads the SPA search state synchronously.
	mustContain := []string{
		"__INITIAL_STATE__",
		"search",
		"feeds",
		"note_card",
		"xsec_token",
		"JSON.stringify(out)",
	}
	for _, w := range mustContain {
		if !strings.Contains(js, w) {
			t.Errorf("searchFeedsJS missing %q", w)
		}
	}
}

func TestScrollPageJS(t *testing.T) {
	// Arrange + Act
	js := scrollPageJS()

	// Assert: scrolls by one viewport.
	if !strings.Contains(js, "window.scrollBy") {
		t.Errorf("scrollPageJS should call window.scrollBy, got %q", js)
	}
}

func TestNoteExtractJS(t *testing.T) {
	// Arrange + Act
	js := noteExtractJS()

	// Assert: reads SSR state and returns stringified JSON.
	mustContain := []string{
		"__INITIAL_STATE__",
		"noteDetailMap",
		"interactInfo",
		"tagList",
		"JSON.stringify(out)",
	}
	for _, w := range mustContain {
		if !strings.Contains(js, w) {
			t.Errorf("noteExtractJS missing %q", w)
		}
	}
}
