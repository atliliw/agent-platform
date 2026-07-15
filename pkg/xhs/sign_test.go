package xhs

import (
	"strings"
	"testing"
)

func TestSearchJS_Interpolation(t *testing.T) {
	// Arrange
	keyword := "羽毛球"
	page := 2
	sort := "time_descending"

	// Act
	js := searchJS(keyword, page, sort)

	// Assert: keyword/page/sort interpolated correctly
	mustContain := []string{
		`var keyword="羽毛球";`,
		"var page=2;",
		`var sort="time_descending";`,
		"https://edith.xiaohongshu.com",
		"/api/sns/web/v1/search/notes",
		"encodeURIComponent(keyword)",
	}
	for _, w := range mustContain {
		if !strings.Contains(js, w) {
			t.Errorf("searchJS missing %q", w)
		}
	}
}

func TestSearchJS_BothAttemptsPresent(t *testing.T) {
	// Arrange + Act
	js := searchJS("test", 1, "general")

	// Assert: the two-tier strategy is encoded - plain fetch first, then sign.
	if !strings.Contains(js, "attempt:'plain'") {
		t.Error("searchJS should try plain fetch first")
	}
	if !strings.Contains(js, "attempt:'signed'") {
		t.Error("searchJS should fall back to explicit sign")
	}
	if !strings.Contains(js, "findSign") {
		t.Error("searchJS should define findSign")
	}
	if !strings.Contains(js, "_webms|sign|xyw|mns") {
		t.Error("searchJS should probe signing function names")
	}
}

func TestSearchJS_Defaults(t *testing.T) {
	// Arrange + Act: page 0 / empty sort should normalize to 1 / general.
	js := searchJS("k", 0, "")

	// Assert
	if !strings.Contains(js, "var page=1;") {
		t.Error("page 0 should default to 1")
	}
	if !strings.Contains(js, `var sort="general";`) {
		t.Error("empty sort should default to general")
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
