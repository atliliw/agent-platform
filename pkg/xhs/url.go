package xhs

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

const (
	// homeURL is the SPA entry used to bootstrap XHS's signing JS and cookies
	// before calling the search API. Lighter than the search page and less
	// likely to trip a captcha gate.
	homeURL       = "https://www.xiaohongshu.com/"
	noteBaseURL   = "https://www.xiaohongshu.com/explore/"
	searchAPIURL  = "https://edith.xiaohongshu.com/api/sns/web/v1/search/notes"
	searchAPIPath = "/api/sns/web/v1/search/notes"
	defaultSort   = "general"
)

// noteIDRe matches XHS note ids (alphanumeric, ~16-32 chars). XHS uses a
// mix of lengths; this is intentionally permissive on length.
var noteIDRe = regexp.MustCompile(`^[a-zA-Z0-9]{16,32}$`)

// HomeURL returns the XHS home URL used to warm the signing context.
func HomeURL() string { return homeURL }

// BuildNoteURL builds the canonical note URL with xsec_token.
func BuildNoteURL(noteID, xsecToken string) string {
	if noteID == "" {
		return ""
	}
	u := noteBaseURL + noteID
	if xsecToken != "" {
		u += "?xsec_token=" + url.QueryEscape(xsecToken) + "&xsec_source=pc_search"
	}
	return u
}

// ParseShareLink extracts the note id and xsec_token from a XHS note URL or
// share link. Accepts:
//
//	https://www.xiaohongshu.com/explore/<id>?xsec_token=...&...
//	https://www.xiaohongshu.com/discovery/item/<id>?xsec_token=...
//	https://www.xiaohongshu.com/note/<id>?xsec_token=...
//	<a bare note id>
//
// Short links (http://xhslink.com/...) cannot be resolved without a fetch and
// return empty values.
func ParseShareLink(s string) (noteID, xsecToken string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	if isNoteID(s) {
		return s, ""
	}
	// Ensure it parses as a URL even if scheme is missing.
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", ""
	}
	id := extractIDFromPath(u.EscapedPath())
	if id == "" {
		return "", ""
	}
	if u.RawQuery != "" {
		q, _ := url.ParseQuery(u.RawQuery)
		xsecToken = q.Get("xsec_token")
	}
	return id, xsecToken
}

func isNoteID(s string) bool {
	return noteIDRe.MatchString(s)
}

// extractIDFromPath pulls the note id out of a path like /explore/<id> or
// /discovery/item/<id>.
func extractIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		if (p == "explore" || p == "discovery" || p == "item" || p == "note") && i+1 < len(parts) {
			if isNoteID(parts[i+1]) {
				return parts[i+1]
			}
		}
	}
	if len(parts) > 0 && isNoteID(parts[len(parts)-1]) {
		return parts[len(parts)-1]
	}
	return ""
}

// BuildSearchAPIURL builds the search API URL with query params. searchID is
// required by XHS; generate one (uuid-ish) on the caller side.
func BuildSearchAPIURL(keyword string, page int, sort, searchID string) string {
	if page < 1 {
		page = 1
	}
	if sort == "" {
		sort = defaultSort
	}
	q := url.Values{}
	q.Set("keyword", keyword)
	q.Set("page", strconv.Itoa(page))
	q.Set("page_size", "20")
	q.Set("search_id", searchID)
	q.Set("sort", sort)
	q.Set("note_type", "0")
	return searchAPIURL + "?" + q.Encode()
}

// SearchAPIPath returns the API path used by the in-page signer.
func SearchAPIPath() string { return searchAPIPath }

// BuildSearchPageURL builds the XHS search_result page URL. Navigating here
// makes the page's own JS fire the signed search XHR during load, so this is
// how a search is actually performed (Obscura runs the page's own async during
// navigation, but not async scheduled via CDP Evaluate).
func BuildSearchPageURL(keyword string) string {
	q := url.Values{}
	q.Set("keyword", keyword)
	q.Set("source", "web_search_result_notes")
	return "https://www.xiaohongshu.com/search_result?" + q.Encode()
}

// IsValidSort reports whether sort is a supported XHS sort value.
func IsValidSort(sort string) bool {
	switch sort {
	case "", "general", "time_descending", "popularity_descending":
		return true
	}
	return false
}
