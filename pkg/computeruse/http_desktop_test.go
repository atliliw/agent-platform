package computeruse

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// recordingServer records the last request to each path and returns 200.
type recordingServer struct {
	mu       map[string][]byte // path -> last request body
	lastPath string
	body     []byte
	png      []byte
	size     string
}

func newRecordingServer() *recordingServer {
	return &recordingServer{png: []byte("\x89PNG fake"), size: `{"width":1920,"height":1080}`}
}

func (r *recordingServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		r.lastPath = req.URL.Path
		r.body, _ = io.ReadAll(req.Body)
		switch req.URL.Path {
		case "/screenshot":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(r.png)
		case "/screen_size":
			_, _ = w.Write([]byte(r.size))
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
}

func TestHTTPDesktopScreenshot(t *testing.T) {
	srv := newRecordingServer()
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	d := NewHTTPDesktop(httpSrv.URL)
	img, err := d.Screenshot(context.Background())
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	if string(img) != string(srv.png) {
		t.Errorf("got %q, want %q", img, srv.png)
	}
	if srv.lastPath != "/screenshot" {
		t.Errorf("hit %q, want /screenshot", srv.lastPath)
	}
}

func TestHTTPDesktopScreenSize(t *testing.T) {
	srv := newRecordingServer()
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	d := NewHTTPDesktop(httpSrv.URL)
	w, h, err := d.ScreenSize(context.Background())
	if err != nil {
		t.Fatalf("ScreenSize: %v", err)
	}
	if w != 1920 || h != 1080 {
		t.Errorf("got %dx%d, want 1920x1080", w, h)
	}
}

func TestHTTPDesktopClickPostsCoordinate(t *testing.T) {
	srv := newRecordingServer()
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	d := NewHTTPDesktop(httpSrv.URL)
	if err := d.Click(context.Background(), 42, 99); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if srv.lastPath != "/click" {
		t.Errorf("hit %q, want /click", srv.lastPath)
	}
	var body map[string]int
	if err := json.Unmarshal(srv.body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["x"] != 42 || body["y"] != 99 {
		t.Errorf("body = %v, want x=42 y=99", body)
	}
}

func TestHTTPDesktopTypePostsText(t *testing.T) {
	srv := newRecordingServer()
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	d := NewHTTPDesktop(httpSrv.URL)
	if err := d.Type(context.Background(), "hello"); err != nil {
		t.Fatalf("Type: %v", err)
	}
	if srv.lastPath != "/type" {
		t.Errorf("hit %q, want /type", srv.lastPath)
	}
	var body map[string]string
	_ = json.Unmarshal(srv.body, &body)
	if body["text"] != "hello" {
		t.Errorf("body = %v, want text=hello", body)
	}
}

func TestHTTPDesktopKeyPostsCombo(t *testing.T) {
	srv := newRecordingServer()
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	d := NewHTTPDesktop(httpSrv.URL)
	if err := d.Key(context.Background(), "ctrl+c"); err != nil {
		t.Fatalf("Key: %v", err)
	}
	if srv.lastPath != "/key" {
		t.Errorf("hit %q, want /key", srv.lastPath)
	}
	var body map[string]string
	_ = json.Unmarshal(srv.body, &body)
	if body["key"] != "ctrl+c" {
		t.Errorf("body = %v, want key=ctrl+c", body)
	}
}

func TestHTTPDesktopScrollPostsAllFields(t *testing.T) {
	srv := newRecordingServer()
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	d := NewHTTPDesktop(httpSrv.URL)
	if err := d.Scroll(context.Background(), 10, 20, "down", 3); err != nil {
		t.Fatalf("Scroll: %v", err)
	}
	if srv.lastPath != "/scroll" {
		t.Errorf("hit %q, want /scroll", srv.lastPath)
	}
	var body map[string]interface{}
	_ = json.Unmarshal(srv.body, &body)
	if body["direction"] != "down" {
		t.Errorf("direction = %v, want down", body["direction"])
	}
	if body["clicks"].(float64) != 3 {
		t.Errorf("clicks = %v, want 3", body["clicks"])
	}
}

func TestHTTPDesktopLaunchAppPostsName(t *testing.T) {
	srv := newRecordingServer()
	httpSrv := httptest.NewServer(srv.handler())
	defer httpSrv.Close()

	d := NewHTTPDesktop(httpSrv.URL)
	if err := d.LaunchApp(context.Background(), "firefox"); err != nil {
		t.Fatalf("LaunchApp: %v", err)
	}
	if srv.lastPath != "/launch_app" {
		t.Errorf("hit %q, want /launch_app", srv.lastPath)
	}
	var body map[string]string
	_ = json.Unmarshal(srv.body, &body)
	if body["app"] != "firefox" {
		t.Errorf("body = %v, want app=firefox", body)
	}
}

func TestHTTPDesktopErrorPropagates(t *testing.T) {
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no display", http.StatusInternalServerError)
	}))
	defer httpSrv.Close()

	d := NewHTTPDesktop(httpSrv.URL)
	if err := d.Click(context.Background(), 1, 1); err == nil {
		t.Fatal("expected error when sidecar returns 500")
	}
}
