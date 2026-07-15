package xhs

import (
	"encoding/json"
	"fmt"
	"strings"
)

// searchAPIResponse mirrors the XHS search/notes API JSON. Field names match
// the live API (snake_case), not __INITIAL_STATE__ (camelCase).
type searchAPIResponse struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Data    struct {
		Items []struct {
			ID        string          `json:"id"`
			ModelType string          `json:"model_type"`
			XSecToken string          `json:"xsec_token"`
			NoteCard  json.RawMessage `json:"note_card"`
		} `json:"items"`
		HasMore string `json:"has_more"`
	} `json:"data"`
}

// noteCardRaw is the relevant slice of each search item's note_card.
type noteCardRaw struct {
	NoteID       string `json:"note_id"`
	DisplayTitle string `json:"display_title"`
	Type         string `json:"type"`
	User         struct {
		Nickname string `json:"nickname"`
	} `json:"user"`
	InteractInfo struct {
		LikedCount string `json:"liked_count"`
	} `json:"interact_info"`
}

// SearchDiagnostics carries the anti-bot diagnostics surfaced from the in-page
// fetch attempt, so the caller can explain a failure rather than guess.
type SearchDiagnostics struct {
	Attempt    string   `json:"attempt"`     // plain | signed | sign_call_failed | ...
	HTTPStatus int      `json:"http_status"` // 0 if the fetch never completed
	XHSCode    int      `json:"xhs_code"`    // XHS error_code from the response body, if any
	SignFn     string   `json:"sign_fn"`     // name of the signing function used, if any
	SignedKeys []string `json:"signed_keys"` // header keys the signer returned
	Note       string   `json:"note"`        // human-readable summary
}

// String renders a compact one-line diagnostic.
func (d SearchDiagnostics) String() string {
	s := fmt.Sprintf("attempt=%s", d.Attempt)
	if d.HTTPStatus != 0 {
		s += fmt.Sprintf(" http=%d", d.HTTPStatus)
	} else {
		s += " http=none"
	}
	s += fmt.Sprintf(" xhs_code=%d", d.XHSCode)
	if d.SignFn != "" {
		s += fmt.Sprintf(" signFn=%s", d.SignFn)
	}
	if d.Note != "" {
		s += " note=" + d.Note
	}
	return s
}

// ParseSearchResponse parses the XHS search API response body into NoteCards
// plus diagnostics. A non-nil error means the body could not be parsed at all
// (truncation / not JSON); a successful parse with zero items still returns a
// nil error so the caller can inspect Diagnostics.
func ParseSearchResponse(body string) ([]NoteCard, SearchDiagnostics, error) {
	diag := SearchDiagnostics{HTTPStatus: 200}
	var resp searchAPIResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, diag, fmt.Errorf("parse search response: %w (body head: %s)", err, head(body, 200))
	}
	diag.XHSCode = resp.Code
	if !resp.Success {
		diag.Note = fmt.Sprintf("API returned success=false (code=%d msg=%q)", resp.Code, resp.Msg)
	}

	cards := make([]NoteCard, 0, len(resp.Data.Items))
	for _, it := range resp.Data.Items {
		var nc noteCardRaw
		if len(it.NoteCard) > 0 {
			_ = json.Unmarshal(it.NoteCard, &nc) // best-effort; card may be absent on ads
		}
		id := nc.NoteID
		if id == "" {
			id = it.ID
		}
		if id == "" {
			continue
		}
		cards = append(cards, NoteCard{
			ID:        id,
			Title:     nc.DisplayTitle,
			XSecToken: it.XSecToken,
			Type:      nc.Type,
			Author:    nc.User.Nickname,
			Liked:     nc.InteractInfo.LikedCount,
			URL:       BuildNoteURL(id, it.XSecToken),
		})
	}
	if diag.Note == "" {
		diag.Note = fmt.Sprintf("parsed %d items, has_more=%s", len(cards), resp.Data.HasMore)
	}
	return cards, diag, nil
}

// head returns the first n runes of s, single-lined, for error context.
func head(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
