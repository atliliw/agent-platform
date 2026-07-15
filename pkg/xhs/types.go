package xhs

// Note is a structured Xiaohongshu note, the output of ReadNote.
type Note struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Desc    string   `json:"desc"`
	Author  string   `json:"author"`
	Type    string   `json:"type"` // normal | video
	Liked   string   `json:"liked"`
	Comment string   `json:"comment"`
	Tags    []string `json:"tags"`
	URL     string   `json:"url"`
}

// NoteCard is one item in a keyword search result list. XSecToken lets the
// caller immediately ReadNote the card.
type NoteCard struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	XSecToken string `json:"xsec_token"`
	Type      string `json:"type"`
	Author    string `json:"author"`
	Liked     string `json:"liked"`
	URL       string `json:"url"`
}

// SearchResult is the output of Search.
type SearchResult struct {
	Keyword string     `json:"keyword"`
	Page    int        `json:"page"`
	Count   int        `json:"count"`
	Notes   []NoteCard `json:"notes"`
	// Diagnostic carries anti-bot diagnostics (attempt type, HTTP status, XHS
	// error_code) so failures are explainable rather than silent empties.
	Diagnostic string `json:"diagnostic,omitempty"`
}
