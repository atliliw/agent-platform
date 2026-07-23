// Package computeruse provides desktop control (screenshot + mouse + keyboard)
// driven by a vision LLM. It extends the pkg/browseragent decision loop from a
// single web page to the whole desktop: each step the agent takes a screenshot,
// the VLM looks at it and emits one JSON action, the agent executes it, and the
// loop repeats until the VLM signals "done" or the step budget is exhausted.
//
// The Desktop primitives are an interface backed by xdotool + scrot, so the
// agent loop and action routing can be unit-tested without an X server.
package computeruse

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Desktop is the set of primitives the agent can perform on a virtual desktop.
type Desktop interface {
	// Screenshot captures the current screen as PNG bytes.
	Screenshot(ctx context.Context) ([]byte, error)
	// ScreenSize returns the display width and height in pixels (the coordinate
	// space the VLM's coordinates are expressed in).
	ScreenSize(ctx context.Context) (int, int, error)
	MouseMove(ctx context.Context, x, y int) error
	Click(ctx context.Context, x, y int) error
	DoubleClick(ctx context.Context, x, y int) error
	RightClick(ctx context.Context, x, y int) error
	Type(ctx context.Context, text string) error
	// Key presses a key combo in xdotool syntax, e.g. "Return", "Escape",
	// "ctrl+c", "alt+Tab".
	Key(ctx context.Context, combo string) error
	// Scroll clicks the scroll wheel at (x,y). direction is "up" or "down";
	// clicks is the number of scroll notches (default 1).
	Scroll(ctx context.Context, x, y int, direction string, clicks int) error
	// LaunchApp starts an application by name, detached.
	LaunchApp(ctx context.Context, name string) error
	Close() error
}

// ActionType enumerates the desktop actions the VLM may choose.
type ActionType string

const (
	ActionScreenshot  ActionType = "screenshot"
	ActionMouseMove   ActionType = "mouse_move"
	ActionClick       ActionType = "click"
	ActionDoubleClick ActionType = "double_click"
	ActionRightClick  ActionType = "right_click"
	ActionTypeText    ActionType = "type"
	ActionKey         ActionType = "key"
	ActionScroll      ActionType = "scroll"
	ActionLaunchApp   ActionType = "launch_app"
	ActionDone        ActionType = "done"
)

// Action is a single desktop action emitted by the VLM. Coordinates use
// [x, y] in screen pixels.
type Action struct {
	Type      ActionType `json:"action"`
	Coordinate []int     `json:"coordinate,omitempty"` // [x, y]
	Text      string     `json:"text,omitempty"`
	Key       string     `json:"key,omitempty"`
	Direction string     `json:"direction,omitempty"` // "up" | "down"
	Clicks    int        `json:"clicks,omitempty"`
	App       string     `json:"app,omitempty"`
	Result    string     `json:"result,omitempty"`
}

// commandRunner runs a shell command and returns its stdout. It is an interface
// so tests can fake xdotool/scrot without a real X server.
type commandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// osCommandRunner runs commands via os/exec.
type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

// LocalDesktop controls a desktop on the same host by shelling out to xdotool
// (mouse/keyboard) and scrot (screenshots). It requires an X server (real or
// Xvfb). Each method is a stateless exec, so Close is a no-op.
type LocalDesktop struct {
	run           commandRunner
	screenshotCmd string
	tmpDir        string
}

// NewLocalDesktop creates a desktop backed by local xdotool + scrot.
func NewLocalDesktop() *LocalDesktop {
	return &LocalDesktop{
		run:           osCommandRunner{},
		screenshotCmd: "scrot",
		tmpDir:        os.TempDir(),
	}
}

// Screenshot captures the screen via scrot into a temp file and returns the PNG.
func (d *LocalDesktop) Screenshot(ctx context.Context) ([]byte, error) {
	path := filepath.Join(d.tmpDir, fmt.Sprintf("cu-shot-%d.png", time.Now().UnixNano()))
	if _, err := d.run.Run(ctx, d.screenshotCmd, path); err != nil {
		return nil, fmt.Errorf("scrot: %w", err)
	}
	defer os.Remove(path)
	return os.ReadFile(path)
}

// ScreenSize returns the display geometry via `xdotool getdisplaygeometry`.
func (d *LocalDesktop) ScreenSize(ctx context.Context) (int, int, error) {
	out, err := d.run.Run(ctx, "xdotool", "getdisplaygeometry")
	if err != nil {
		return 0, 0, fmt.Errorf("getdisplaygeometry: %w", err)
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("unexpected getdisplaygeometry output: %q", string(out))
	}
	w, errW := strconv.Atoi(parts[0])
	h, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil {
		return 0, 0, fmt.Errorf("parse geometry %q: invalid number", string(out))
	}
	return w, h, nil
}

// MouseMove moves the pointer to (x, y).
func (d *LocalDesktop) MouseMove(ctx context.Context, x, y int) error {
	_, err := d.run.Run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y))
	return err
}

// Click moves to (x, y) and left-clicks.
func (d *LocalDesktop) Click(ctx context.Context, x, y int) error {
	_, err := d.run.Run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y), "click", "1")
	return err
}

// DoubleClick moves to (x, y) and double-clicks.
func (d *LocalDesktop) DoubleClick(ctx context.Context, x, y int) error {
	_, err := d.run.Run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y),
		"click", "--repeat", "2", "--delay", "100", "1")
	return err
}

// RightClick moves to (x, y) and right-clicks.
func (d *LocalDesktop) RightClick(ctx context.Context, x, y int) error {
	_, err := d.run.Run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y), "click", "3")
	return err
}

// Type types a string at the current focus, with a small per-key delay.
func (d *LocalDesktop) Type(ctx context.Context, text string) error {
	_, err := d.run.Run(ctx, "xdotool", "type", "--delay", "12", "--", text)
	return err
}

// Key presses a key combo (xdotool syntax).
func (d *LocalDesktop) Key(ctx context.Context, combo string) error {
	if combo == "" {
		return fmt.Errorf("empty key combo")
	}
	_, err := d.run.Run(ctx, "xdotool", "key", combo)
	return err
}

// Scroll scrolls at (x, y). direction "up" -> button 4, "down" -> button 5.
func (d *LocalDesktop) Scroll(ctx context.Context, x, y int, direction string, clicks int) error {
	if clicks <= 0 {
		clicks = 1
	}
	button := "5"
	if direction == "up" {
		button = "4"
	}
	_, err := d.run.Run(ctx, "xdotool", "mousemove", "--sync", strconv.Itoa(x), strconv.Itoa(y),
		"click", "--repeat", strconv.Itoa(clicks), button)
	return err
}

// LaunchApp starts an application detached. The name is passed as a positional
// shell argument (not interpolated) to avoid command injection from VLM output.
func (d *LocalDesktop) LaunchApp(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("empty app name")
	}
	// `sh -c '... &'` returns immediately after backgrounding; $1 is the app.
	_, err := d.run.Run(ctx, "sh", "-c", "$1 >/dev/null 2>&1 &", "launch", name)
	return err
}

// Close releases resources. LocalDesktop is stateless, so this is a no-op.
func (d *LocalDesktop) Close() error { return nil }

// executeAction performs an action on the desktop and returns a short result
// string for the next prompt (or an error if the action failed or was invalid).
func executeAction(ctx context.Context, desk Desktop, action *Action) (string, error) {
	switch action.Type {
	case ActionScreenshot:
		// Screenshots are taken every step regardless; this is a no-op hint.
		return "已截图", nil
	case ActionMouseMove:
		if len(action.Coordinate) < 2 {
			return "", fmt.Errorf("mouse_move requires coordinate [x,y]")
		}
		if err := desk.MouseMove(ctx, action.Coordinate[0], action.Coordinate[1]); err != nil {
			return "", err
		}
		return fmt.Sprintf("移动到 (%d,%d)", action.Coordinate[0], action.Coordinate[1]), nil
	case ActionClick:
		if len(action.Coordinate) < 2 {
			return "", fmt.Errorf("click requires coordinate [x,y]")
		}
		if err := desk.Click(ctx, action.Coordinate[0], action.Coordinate[1]); err != nil {
			return "", err
		}
		return fmt.Sprintf("左键点击 (%d,%d)", action.Coordinate[0], action.Coordinate[1]), nil
	case ActionDoubleClick:
		if len(action.Coordinate) < 2 {
			return "", fmt.Errorf("double_click requires coordinate [x,y]")
		}
		if err := desk.DoubleClick(ctx, action.Coordinate[0], action.Coordinate[1]); err != nil {
			return "", err
		}
		return fmt.Sprintf("双击 (%d,%d)", action.Coordinate[0], action.Coordinate[1]), nil
	case ActionRightClick:
		if len(action.Coordinate) < 2 {
			return "", fmt.Errorf("right_click requires coordinate [x,y]")
		}
		if err := desk.RightClick(ctx, action.Coordinate[0], action.Coordinate[1]); err != nil {
			return "", err
		}
		return fmt.Sprintf("右键点击 (%d,%d)", action.Coordinate[0], action.Coordinate[1]), nil
	case ActionTypeText:
		if action.Text == "" {
			return "", fmt.Errorf("type requires text")
		}
		if err := desk.Type(ctx, action.Text); err != nil {
			return "", err
		}
		return fmt.Sprintf("输入 %q", action.Text), nil
	case ActionKey:
		if err := desk.Key(ctx, action.Key); err != nil {
			return "", err
		}
		return fmt.Sprintf("按键 %s", action.Key), nil
	case ActionScroll:
		if len(action.Coordinate) < 2 {
			return "", fmt.Errorf("scroll requires coordinate [x,y]")
		}
		if err := desk.Scroll(ctx, action.Coordinate[0], action.Coordinate[1], action.Direction, action.Clicks); err != nil {
			return "", err
		}
		return fmt.Sprintf("滚动 %s %d 次", action.Direction, action.Clicks), nil
	case ActionLaunchApp:
		if err := desk.LaunchApp(ctx, action.App); err != nil {
			return "", err
		}
		return fmt.Sprintf("启动应用 %q", action.App), nil
	case ActionDone:
		return "", nil
	default:
		return "", fmt.Errorf("unknown action: %s", action.Type)
	}
}
