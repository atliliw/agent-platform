package computeruse

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// fakeDesktop records every primitive call so tests can assert routing.
type fakeDesktop struct {
	mu           sync.Mutex
	shots        int
	screenW      int
	screenH      int
	moves        [][2]int
	clicks       [][2]int
	doubleClicks [][2]int
	rightClicks  [][2]int
	typed        []string
	keys         []string
	scrolls      []scrollCall
	apps         []string
	shotErr      error
	closeCount   int
}

type scrollCall struct {
	x, y   int
	dir    string
	clicks int
}

func (f *fakeDesktop) Screenshot(_ context.Context) ([]byte, error) {
	f.mu.Lock()
	f.shots++
	f.mu.Unlock()
	if f.shotErr != nil {
		return nil, f.shotErr
	}
	return []byte("png-bytes"), nil
}

func (f *fakeDesktop) ScreenSize(_ context.Context) (int, int, error) {
	return f.screenW, f.screenH, nil
}
func (f *fakeDesktop) MouseMove(_ context.Context, x, y int) error {
	f.mu.Lock(); defer f.mu.Unlock(); f.moves = append(f.moves, [2]int{x, y}); return nil
}
func (f *fakeDesktop) Click(_ context.Context, x, y int) error {
	f.mu.Lock(); defer f.mu.Unlock(); f.clicks = append(f.clicks, [2]int{x, y}); return nil
}
func (f *fakeDesktop) DoubleClick(_ context.Context, x, y int) error {
	f.mu.Lock(); defer f.mu.Unlock(); f.doubleClicks = append(f.doubleClicks, [2]int{x, y}); return nil
}
func (f *fakeDesktop) RightClick(_ context.Context, x, y int) error {
	f.mu.Lock(); defer f.mu.Unlock(); f.rightClicks = append(f.rightClicks, [2]int{x, y}); return nil
}
func (f *fakeDesktop) Type(_ context.Context, text string) error {
	f.mu.Lock(); defer f.mu.Unlock(); f.typed = append(f.typed, text); return nil
}
func (f *fakeDesktop) Key(_ context.Context, combo string) error {
	f.mu.Lock(); defer f.mu.Unlock(); f.keys = append(f.keys, combo); return nil
}
func (f *fakeDesktop) Scroll(_ context.Context, x, y int, dir string, clicks int) error {
	f.mu.Lock(); defer f.mu.Unlock(); f.scrolls = append(f.scrolls, scrollCall{x, y, dir, clicks}); return nil
}
func (f *fakeDesktop) LaunchApp(_ context.Context, name string) error {
	f.mu.Lock(); defer f.mu.Unlock(); f.apps = append(f.apps, name); return nil
}
func (f *fakeDesktop) Close() error {
	f.mu.Lock(); defer f.mu.Unlock(); f.closeCount++; return nil
}

// scriptVLM returns a scripted sequence of responses and records screenshots.
type scriptVLM struct {
	responses []string
	shots     [][]byte
	calls     int
	mu        sync.Mutex
}

func (v *scriptVLM) Chat(_ context.Context, _, _ string, shot []byte) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.calls++
	v.shots = append(v.shots, shot)
	idx := v.calls - 1
	if idx < len(v.responses) {
		return v.responses[idx], nil
	}
	// Exhausted: repeat the last response so max-step tests run to the limit.
	if len(v.responses) > 0 {
		return v.responses[len(v.responses)-1], nil
	}
	return `{"action":"done","result":"exhausted"}`, nil
}

// ---- executeAction routing ----

func TestExecuteActionRoutesClick(t *testing.T) {
	// Arrange
	desk := &fakeDesktop{}
	action := &Action{Type: ActionClick, Coordinate: []int{10, 20}}

	// Act
	res, err := executeAction(context.Background(), desk, action)

	// Assert
	if err != nil {
		t.Fatalf("executeAction: %v", err)
	}
	if len(desk.clicks) != 1 || desk.clicks[0] != [2]int{10, 20} {
		t.Errorf("expected click at (10,20), got %v", desk.clicks)
	}
	if res == "" {
		t.Error("expected non-empty result string")
	}
}

func TestExecuteActionValidatesClickCoordinate(t *testing.T) {
	desk := &fakeDesktop{}
	_, err := executeAction(context.Background(), desk, &Action{Type: ActionClick})
	if err == nil {
		t.Fatal("expected error for click without coordinate")
	}
}

func TestExecuteActionTypeRoutes(t *testing.T) {
	desk := &fakeDesktop{}
	_, err := executeAction(context.Background(), desk, &Action{Type: ActionTypeText, Text: "hello"})
	if err != nil {
		t.Fatalf("executeAction: %v", err)
	}
	if len(desk.typed) != 1 || desk.typed[0] != "hello" {
		t.Errorf("expected type 'hello', got %v", desk.typed)
	}
}

func TestExecuteActionUnknownType(t *testing.T) {
	desk := &fakeDesktop{}
	_, err := executeAction(context.Background(), desk, &Action{Type: ActionType("frobnicate")})
	if err == nil {
		t.Fatal("expected error for unknown action type")
	}
}

func TestExecuteActionDoneIsNoop(t *testing.T) {
	desk := &fakeDesktop{}
	res, err := executeAction(context.Background(), desk, &Action{Type: ActionDone, Result: "ok"})
	if err != nil {
		t.Fatalf("executeAction done: %v", err)
	}
	if res != "" {
		t.Errorf("done should be a no-op, got result %q", res)
	}
	if len(desk.clicks)+len(desk.typed)+len(desk.moves) != 0 {
		t.Error("done should not trigger any desktop primitive")
	}
}

// ---- Agent.Run loop ----

func TestAgentLoopExecutesAndStopsOnDone(t *testing.T) {
	// Arrange
	desk := &fakeDesktop{screenW: 1920, screenH: 1080}
	vlm := &scriptVLM{responses: []string{
		`{"action":"click","coordinate":[10,20]}`,
		`{"action":"type","text":"hi"}`,
		`{"action":"done","result":"finished"}`,
	}}
	a := New(vlm, desk)

	// Act
	res, err := a.Run(context.Background(), "open calculator and compute 1+1")

	// Assert
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Success {
		t.Fatal("expected success")
	}
	if res.Answer != "finished" {
		t.Errorf("Answer = %q, want %q", res.Answer, "finished")
	}
	if res.Steps != 3 {
		t.Errorf("Steps = %d, want 3", res.Steps)
	}
	if len(desk.clicks) != 1 || desk.clicks[0] != [2]int{10, 20} {
		t.Errorf("expected one click at (10,20), got %v", desk.clicks)
	}
	if len(desk.typed) != 1 || desk.typed[0] != "hi" {
		t.Errorf("expected type 'hi', got %v", desk.typed)
	}
	// The VLM must have received a screenshot on every step.
	if len(vlm.shots) != 3 {
		t.Errorf("expected 3 screenshots forwarded to VLM, got %d", len(vlm.shots))
	}
	for i, s := range vlm.shots {
		if len(s) == 0 {
			t.Errorf("step %d: screenshot not forwarded to VLM", i+1)
		}
	}
}

func TestAgentLoopStopsOnMaxSteps(t *testing.T) {
	// Arrange: VLM never emits done.
	desk := &fakeDesktop{screenW: 1920, screenH: 1080}
	vlm := &scriptVLM{responses: []string{`{"action":"mouse_move","coordinate":[0,0]}`}}
	a := New(vlm, desk, WithMaxSteps(3))

	// Act
	res, err := a.Run(context.Background(), "endless task")

	// Assert
	if !errors.Is(err, ErrMaxStepsReached) {
		t.Fatalf("expected ErrMaxStepsReached, got %v", err)
	}
	if res.Success {
		t.Error("expected failure on max steps")
	}
	if res.Steps != 3 {
		t.Errorf("Steps = %d, want 3", res.Steps)
	}
	if len(desk.moves) != 3 {
		t.Errorf("expected 3 mouse moves, got %d", len(desk.moves))
	}
}

func TestAgentLoopRecoversFromUnparseableResponse(t *testing.T) {
	// Arrange: first response is garbage, second is done.
	desk := &fakeDesktop{screenW: 1920, screenH: 1080}
	vlm := &scriptVLM{responses: []string{
		"the screen shows a calculator", // no JSON
		`{"action":"done","result":"ok"}`,
	}}
	a := New(vlm, desk)

	// Act
	res, err := a.Run(context.Background(), "task")

	// Assert
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Success {
		t.Fatal("expected recovery and success")
	}
	if res.Steps != 2 {
		t.Errorf("Steps = %d, want 2 (1 unparseable + 1 done)", res.Steps)
	}
}

func TestAgentLoopAbortsOnScreenshotError(t *testing.T) {
	// Arrange
	desk := &fakeDesktop{shotErr: errors.New("display gone")}
	vlm := &scriptVLM{responses: []string{`{"action":"done","result":"x"}`}}
	a := New(vlm, desk)

	// Act
	_, err := a.Run(context.Background(), "task")

	// Assert
	if err == nil {
		t.Fatal("expected error when screenshot fails")
	}
}

// ---- ActionGate (per-action approval) ----

// selectiveGate rejects one action type and approves everything else.
type selectiveGate struct {
	rejectType ActionType
	reason     string
	err        error
}

func (g *selectiveGate) Approve(_ context.Context, action *Action) (bool, string, error) {
	if g.err != nil {
		return false, "", g.err
	}
	if action.Type == g.rejectType {
		return false, g.reason, nil
	}
	return true, "", nil
}

func TestAgentLoopGateRejectsLaunchApp(t *testing.T) {
	// Arrange: VLM tries to launch an app, then gives up. The gate rejects
	// launch_app, so it must be skipped (not executed on the desktop).
	desk := &fakeDesktop{screenW: 1920, screenH: 1080}
	vlm := &scriptVLM{responses: []string{
		`{"action":"launch_app","app":"firefox"}`,
		`{"action":"done","result":"cancelled by user"}`,
	}}
	gate := &selectiveGate{rejectType: ActionLaunchApp, reason: "launching apps requires approval"}
	a := New(vlm, desk, WithGate(gate))

	// Act
	res, err := a.Run(context.Background(), "launch firefox")

	// Assert
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Success {
		t.Fatal("expected success after rejection + done")
	}
	if len(desk.apps) != 0 {
		t.Errorf("launch_app should have been skipped, but apps=%v", desk.apps)
	}
	if res.Steps != 2 {
		t.Errorf("Steps = %d, want 2", res.Steps)
	}
}

func TestAgentLoopGateApprovesBenignAction(t *testing.T) {
	// Arrange: gate only rejects launch_app, so click is approved and executed.
	desk := &fakeDesktop{screenW: 1920, screenH: 1080}
	vlm := &scriptVLM{responses: []string{
		`{"action":"click","coordinate":[5, 6]}`,
		`{"action":"done","result":"ok"}`,
	}}
	gate := &selectiveGate{rejectType: ActionLaunchApp}
	a := New(vlm, desk, WithGate(gate))

	// Act
	res, err := a.Run(context.Background(), "click something")

	// Assert
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Success {
		t.Fatal("expected success")
	}
	if len(desk.clicks) != 1 || desk.clicks[0] != [2]int{5, 6} {
		t.Errorf("approved click should execute, got clicks=%v", desk.clicks)
	}
}

func TestAgentLoopGateErrorAborts(t *testing.T) {
	// Arrange: gate returns a hard error -> the run must abort.
	desk := &fakeDesktop{screenW: 1920, screenH: 1080}
	vlm := &scriptVLM{responses: []string{`{"action":"click","coordinate":[1, 2]}`}}
	gate := &selectiveGate{err: errors.New("approval service unavailable")}
	a := New(vlm, desk, WithGate(gate))

	// Act
	_, err := a.Run(context.Background(), "task")

	// Assert
	if err == nil {
		t.Fatal("expected error when approval gate fails")
	}
}
