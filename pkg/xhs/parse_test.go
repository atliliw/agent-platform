package xhs

import (
	"strings"
	"testing"
)

const searchFixture = `{
  "success": true, "code": 0, "msg": "成功",
  "data": {
    "items": [
      {"id":"64a1b2c3d4e5f6a7b8c9d0e1","model_type":"note","xsec_token":"ABtoken1",
       "note_card":{"note_id":"64a1b2c3d4e5f6a7b8c9d0e1","display_title":"羽毛球好物分享","type":"normal",
         "user":{"nickname":"球友小王"},"interact_info":{"liked_count":"1234"}}},
      {"id":"64a1b2c3d4e5f6a7b8c9d0e2","model_type":"note","xsec_token":"ABtoken2",
       "note_card":{"note_id":"64a1b2c3d4e5f6a7b8c9d0e2","display_title":"周末打球vlog","type":"video",
         "user":{"nickname":"运动达人"},"interact_info":{"liked_count":"56"}}}
    ],
    "has_more": "true"
  }
}`

func TestParseSearchResponse_Success(t *testing.T) {
	// Arrange: a two-item success response (fixture above).

	// Act
	cards, diag, err := ParseSearchResponse(searchFixture)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(cards))
	}
	first := cards[0]
	if first.ID != "64a1b2c3d4e5f6a7b8c9d0e1" {
		t.Errorf("first id = %q", first.ID)
	}
	if first.Title != "羽毛球好物分享" {
		t.Errorf("first title = %q", first.Title)
	}
	if first.XSecToken != "ABtoken1" {
		t.Errorf("first xsec_token = %q", first.XSecToken)
	}
	if first.Author != "球友小王" || first.Liked != "1234" || first.Type != "normal" {
		t.Errorf("first meta wrong: author=%q liked=%q type=%q", first.Author, first.Liked, first.Type)
	}
	if !strings.Contains(first.URL, "xsec_token=ABtoken1") {
		t.Errorf("first url missing token: %q", first.URL)
	}
	if diag.XHSCode != 0 {
		t.Errorf("diag code = %d, want 0", diag.XHSCode)
	}
}

func TestParseSearchResponse_Blocked(t *testing.T) {
	// Arrange: API returned success=false (e.g. anti-bot / risk control).
	body := `{"success":false,"code":461,"msg":"登录","data":{"items":[]}}`

	// Act
	cards, diag, err := ParseSearchResponse(body)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cards) != 0 {
		t.Errorf("expected 0 cards on blocked response, got %d", len(cards))
	}
	if diag.XHSCode != 461 {
		t.Errorf("diag code = %d, want 461", diag.XHSCode)
	}
	if !strings.Contains(diag.Note, "success=false") {
		t.Errorf("diag note should mention success=false, got %q", diag.Note)
	}
}

func TestParseSearchResponse_Truncated(t *testing.T) {
	// Arrange: body is not valid JSON.
	body := `{"success":true,"data":{"items":[` // truncated

	// Act
	_, _, err := ParseSearchResponse(body)

	// Assert
	if err == nil {
		t.Fatal("expected error on truncated body, got nil")
	}
}

func TestParseSearchResponse_AdItemSkipped(t *testing.T) {
	// Arrange: an item with no note_card (ad/placeholder) and no id must be
	// skipped without breaking the rest.
	body := `{"success":true,"data":{"items":[
		{"id":"","model_type":"ad"},
		{"id":"64a1b2c3d4e5f6a7b8c9d0e3","model_type":"note","xsec_token":"T",
		 "note_card":{"note_id":"64a1b2c3d4e5f6a7b8c9d0e3","display_title":"正常笔记","type":"normal",
		  "user":{"nickname":"作者"},"interact_info":{"liked_count":"1"}}}
	]}}`

	// Act
	cards, _, err := ParseSearchResponse(body)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cards) != 1 {
		t.Fatalf("expected 1 card (ad skipped), got %d", len(cards))
	}
	if cards[0].Title != "正常笔记" {
		t.Errorf("card title = %q", cards[0].Title)
	}
}
