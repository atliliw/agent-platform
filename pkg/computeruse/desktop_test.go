package computeruse

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// fakeRunner is a commandRunner that records calls and stubs xdotool/scrot so
// LocalDesktop can be tested without an X server.
type fakeRunner struct {
	calls        [][]string
	scrotContent []byte
	geometry     string
	err          error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{name}, args...))
	if f.err != nil {
		return nil, f.err
	}
	switch name {
	case "scrot":
		// args[0] is the output path; create it so Screenshot can read it back.
		if len(args) > 0 {
			_ = os.WriteFile(args[0], f.scrotContent, 0o644)
		}
		return nil, nil
	case "xdotool":
		if len(args) > 0 && args[0] == "getdisplaygeometry" {
			return []byte(f.geometry), nil
		}
		return nil, nil
	case "sh":
		return nil, nil
	}
	return nil, nil
}

func joinArgs(call []string) string { return strings.Join(call, " ") }

func TestScreenshotReturnsPNG(t *testing.T) {
	// Arrange
	d := &LocalDesktop{
		run:           &fakeRunner{scrotContent: []byte("\x89PNG\r\n\x1a\nfake")},
		screenshotCmd: "scrot",
		tmpDir:        os.TempDir(),
	}

	// Act
	img, err := d.Screenshot(context.Background())

	// Assert
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	if !strings.HasPrefix(string(img), "\x89PNG") {
		t.Errorf("expected PNG header, got %q", img)
	}
}

func TestScreenshotErrorPropagates(t *testing.T) {
	d := &LocalDesktop{run: &fakeRunner{err: errors.New("no display")}, screenshotCmd: "scrot", tmpDir: os.TempDir()}
	if _, err := d.Screenshot(context.Background()); err == nil {
		t.Fatal("expected error when scrot fails")
	}
}

func TestScreenSizeParsed(t *testing.T) {
	d := &LocalDesktop{run: &fakeRunner{geometry: "1920 1080\n"}, screenshotCmd: "scrot", tmpDir: os.TempDir()}
	w, h, err := d.ScreenSize(context.Background())
	if err != nil {
		t.Fatalf("ScreenSize: %v", err)
	}
	if w != 1920 || h != 1080 {
		t.Errorf("got %dx%d, want 1920x1080", w, h)
	}
}

func TestScreenSizeRejectsGarbage(t *testing.T) {
	d := &LocalDesktop{run: &fakeRunner{geometry: "nope"}, screenshotCmd: "scrot", tmpDir: os.TempDir()}
	if _, _, err := d.ScreenSize(context.Background()); err == nil {
		t.Fatal("expected error for unparseable geometry")
	}
}

func TestClickIssuesMoveAndClick1(t *testing.T) {
	fr := &fakeRunner{geometry: "1920 1080\n"}
	d := &LocalDesktop{run: fr, screenshotCmd: "scrot", tmpDir: os.TempDir()}

	if err := d.Click(context.Background(), 100, 200); err != nil {
		t.Fatalf("Click: %v", err)
	}
	if len(fr.calls) != 1 {
		t.Fatalf("expected 1 xdotool call, got %d: %v", len(fr.calls), fr.calls)
	}
	got := joinArgs(fr.calls[0])
	if !strings.Contains(got, "mousemove") || !strings.Contains(got, "100") || !strings.Contains(got, "200") || !strings.Contains(got, "click 1") {
		t.Errorf("click command unexpected: %q", got)
	}
}

func TestTypePassesText(t *testing.T) {
	fr := &fakeRunner{}
	d := &LocalDesktop{run: fr, screenshotCmd: "scrot", tmpDir: os.TempDir()}

	if err := d.Type(context.Background(), "hello world"); err != nil {
		t.Fatalf("Type: %v", err)
	}
	got := joinArgs(fr.calls[0])
	if !strings.Contains(got, "hello world") {
		t.Errorf("type did not pass text: %q", got)
	}
	if !strings.Contains(got, "--") {
		t.Errorf("type should use -- to guard text args: %q", got)
	}
}

func TestScrollUpUsesButton4(t *testing.T) {
	fr := &fakeRunner{}
	d := &LocalDesktop{run: fr, screenshotCmd: "scrot", tmpDir: os.TempDir()}

	if err := d.Scroll(context.Background(), 50, 60, "up", 3); err != nil {
		t.Fatalf("Scroll: %v", err)
	}
	got := joinArgs(fr.calls[0])
	if !strings.Contains(got, "4") { // button 4 = up
		t.Errorf("scroll up should use button 4: %q", got)
	}
	if !strings.Contains(got, "3") { // repeat 3
		t.Errorf("scroll should repeat 3: %q", got)
	}
}

func TestScrollDownUsesButton5(t *testing.T) {
	fr := &fakeRunner{}
	d := &LocalDesktop{run: fr, screenshotCmd: "scrot", tmpDir: os.TempDir()}

	_ = d.Scroll(context.Background(), 50, 60, "down", 1)
	got := joinArgs(fr.calls[0])
	if !strings.Contains(got, "5") {
		t.Errorf("scroll down should use button 5: %q", got)
	}
}

func TestKeyRejectsEmpty(t *testing.T) {
	d := &LocalDesktop{run: &fakeRunner{}, screenshotCmd: "scrot", tmpDir: os.TempDir()}
	if err := d.Key(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty key combo")
	}
}

func TestLaunchAppRejectsEmpty(t *testing.T) {
	d := &LocalDesktop{run: &fakeRunner{}, screenshotCmd: "scrot", tmpDir: os.TempDir()}
	if err := d.LaunchApp(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty app name")
	}
}

func TestLaunchAppPassesNameAsPositionalArg(t *testing.T) {
	fr := &fakeRunner{}
	d := &LocalDesktop{run: fr, screenshotCmd: "scrot", tmpDir: os.TempDir()}

	if err := d.LaunchApp(context.Background(), "firefox"); err != nil {
		t.Fatalf("LaunchApp: %v", err)
	}
	if len(fr.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(fr.calls))
	}
	call := fr.calls[0] // ["sh", "-c", "$1 >/dev/null 2>&1 &", "launch", "firefox"]
	if call[0] != "sh" || call[1] != "-c" {
		t.Errorf("expected sh -c wrapper, got %v", call)
	}
	scriptBody := call[2]
	if strings.Contains(scriptBody, "firefox") {
		t.Errorf("app name must NOT be interpolated into script body (injection risk): %q", scriptBody)
	}
	// The app name is passed as a positional arg ($1), not baked into the script.
	last := call[len(call)-1]
	if last != "firefox" {
		t.Errorf("expected app name as final positional arg, got %q (call=%v)", last, call)
	}
}
