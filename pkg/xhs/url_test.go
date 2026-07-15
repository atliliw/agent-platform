package xhs

import (
	"strings"
	"testing"
)

func TestBuildNoteURL(t *testing.T) {
	tests := []struct {
		name      string
		noteID    string
		xsecToken string
		want      string
	}{
		{
			name:      "id + token",
			noteID:    "64a1b2c3d4e5f6a7b8c9d0e1",
			xsecToken: "ABxyz123",
			want:      "https://www.xiaohongshu.com/explore/64a1b2c3d4e5f6a7b8c9d0e1?xsec_token=ABxyz123&xsec_source=pc_search",
		},
		{
			name:   "id only",
			noteID: "64a1b2c3d4e5f6a7b8c9d0e1",
			want:   "https://www.xiaohongshu.com/explore/64a1b2c3d4e5f6a7b8c9d0e1",
		},
		{
			name:      "empty id returns empty",
			noteID:    "",
			xsecToken: "ABxyz",
			want:      "",
		},
		{
			name:      "token url-escaped",
			noteID:    "64a1b2c3d4e5f6a7b8c9d0e1",
			xsecToken: "AB cd+",
			want:      "https://www.xiaohongshu.com/explore/64a1b2c3d4e5f6a7b8c9d0e1?xsec_token=AB+cd%2B&xsec_source=pc_search",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildNoteURL(tt.noteID, tt.xsecToken)
			if got != tt.want {
				t.Errorf("BuildNoteURL(%q,%q) = %q, want %q", tt.noteID, tt.xsecToken, got, tt.want)
			}
		})
	}
}

func TestParseShareLink(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantID      string
		wantToken   string
	}{
		{
			name:      "explore url with token",
			input:     "https://www.xiaohongshu.com/explore/64a1b2c3d4e5f6a7b8c9d0e1?xsec_token=ABxyz&xsec_source=pc_search",
			wantID:    "64a1b2c3d4e5f6a7b8c9d0e1",
			wantToken: "ABxyz",
		},
		{
			name:      "discovery item url",
			input:     "https://www.xiaohongshu.com/discovery/item/64a1b2c3d4e5f6a7b8c9d0e1?xsec_token=TKN",
			wantID:    "64a1b2c3d4e5f6a7b8c9d0e1",
			wantToken: "TKN",
		},
		{
			name:    "bare note id",
			input:   "64a1b2c3d4e5f6a7b8c9d0e1",
			wantID:  "64a1b2c3d4e5f6a7b8c9d0e1",
			wantToken: "",
		},
		{
			name:    "url without token",
			input:   "https://www.xiaohongshu.com/explore/64a1b2c3d4e5f6a7b8c9d0e1",
			wantID:  "64a1b2c3d4e5f6a7b8c9d0e1",
			wantToken: "",
		},
		{
			name:    "short link unresolvable",
			input:   "http://xhslink.com/a/abc",
			wantID:  "",
			wantToken: "",
		},
		{
			name:    "empty input",
			input:   "",
			wantID:  "",
			wantToken: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, token := ParseShareLink(tt.input)
			if id != tt.wantID || token != tt.wantToken {
				t.Errorf("ParseShareLink(%q) = (%q,%q), want (%q,%q)", tt.input, id, token, tt.wantID, tt.wantToken)
			}
		})
	}
}

func TestBuildSearchAPIURL(t *testing.T) {
	// Arrange
	keyword := "羽毛球"
	page := 2
	sort := "time_descending"
	searchID := "11111111-2222-3333-4444-555555555555"

	// Act
	got := BuildSearchAPIURL(keyword, page, sort, searchID)

	// Assert
	wantContains := []string{
		"https://edith.xiaohongshu.com/api/sns/web/v1/search/notes?",
		"keyword=" + "%E7%BE%BD%E6%AF%9B%E7%90%83", // url-encoded 羽毛球
		"page=2",
		"page_size=20",
		"search_id=11111111-2222-3333-4444-555555555555",
		"sort=time_descending",
		"note_type=0",
	}
	for _, w := range wantContains {
		if !strings.Contains(got, w) {
			t.Errorf("BuildSearchAPIURL missing %q in %q", w, got)
		}
	}
}

func TestBuildSearchAPIURLDefaults(t *testing.T) {
	// page<1 -> 1, empty sort -> general
	got := BuildSearchAPIURL("x", 0, "", "sid")
	if !strings.Contains(got, "page=1") {
		t.Errorf("expected page=1 default, got %q", got)
	}
	if !strings.Contains(got, "sort=general") {
		t.Errorf("expected sort=general default, got %q", got)
	}
}

func TestIsValidSort(t *testing.T) {
	valid := []string{"", "general", "time_descending", "popularity_descending"}
	invalid := []string{"hot", "random", "General"}
	for _, s := range valid {
		if !IsValidSort(s) {
			t.Errorf("IsValidSort(%q) = false, want true", s)
		}
	}
	for _, s := range invalid {
		if IsValidSort(s) {
			t.Errorf("IsValidSort(%q) = true, want false", s)
		}
	}
}
