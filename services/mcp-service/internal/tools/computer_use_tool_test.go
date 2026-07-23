package tools

import (
	"context"
	"strings"
	"testing"

	"agent-platform/pkg/computeruse"
)

// fakeVLM is a VLMClient stub that always returns the same response and
// records how many screenshots it received.
type fakeVLM struct {
	resp        string
	chatCalls   int
	shotsSeen   int
}

func (f *fakeVLM) Chat(_ context.Context, _, _ string, shot []byte) (string, error) {
	f.chatCalls++
	if len(shot) > 0 {
		f.shotsSeen++
	}
	return f.resp, nil
}

// fakeDesktop is a computeruse.Desktop stub.
type fakeDesktop struct {
	closed bool
}

func (f *fakeDesktop) Screenshot(_ context.Context) ([]byte, error) { return []byte("png"), nil }
func (f *fakeDesktop) ScreenSize(_ context.Context) (int, int, error) {
	return 1920, 1080, nil
}
func (f *fakeDesktop) MouseMove(_ context.Context, _, _ int) error    { return nil }
func (f *fakeDesktop) Click(_ context.Context, _, _ int) error        { return nil }
func (f *fakeDesktop) DoubleClick(_ context.Context, _, _ int) error  { return nil }
func (f *fakeDesktop) RightClick(_ context.Context, _, _ int) error   { return nil }
func (f *fakeDesktop) Type(_ context.Context, _ string) error         { return nil }
func (f *fakeDesktop) Key(_ context.Context, _ string) error          { return nil }
func (f *fakeDesktop) Scroll(_ context.Context, _, _ int, _ string, _ int) error {
	return nil
}
func (f *fakeDesktop) LaunchApp(_ context.Context, _ string) error { return nil }
func (f *fakeDesktop) Close() error                                 { f.closed = true; return nil }

func newTestComputerUseTool(vlm computeruse.VLMClient, desk computeruse.Desktop) *ComputerUseTool {
	t := NewComputerUseToolWithConfig("test-key", "http://vlm.local", "")
	t.vlmFactory = func(_, _, _ string) computeruse.VLMClient { return vlm }
	t.desktopFactory = func() (computeruse.Desktop, error) { return desk, nil }
	return t
}

func TestComputerUseToolRequiresTask(t *testing.T) {
	tool := newTestComputerUseTool(&fakeVLM{resp: `{"action":"done","result":"x"}`}, &fakeDesktop{})
	_, err := tool.ExecuteWithConfig(context.Background(), map[string]interface{}{}, nil)
	if err == nil {
		t.Fatal("expected error when task is missing")
	}
}

func TestComputerUseToolRequiresAPIKey(t *testing.T) {
	// No API key in struct or config.
	tool := NewComputerUseTool()
	_, err := tool.ExecuteWithConfig(context.Background(),
		map[string]interface{}{"task": "do something"}, nil)
	if err == nil {
		t.Fatal("expected error when API key is not configured")
	}
	if !strings.Contains(err.Error(), "API Key") {
		t.Errorf("expected API key error, got %v", err)
	}
}

func TestComputerUseToolRunsAndReturnsAnswer(t *testing.T) {
	// Arrange: VLM immediately signals done with an answer.
	vlm := &fakeVLM{resp: `{"action":"done","result":"1+1=2"}`}
	desk := &fakeDesktop{}
	tool := newTestComputerUseTool(vlm, desk)

	// Act
	out, err := tool.ExecuteWithConfig(context.Background(),
		map[string]interface{}{"task": "compute 1+1"}, nil)

	// Assert
	if err != nil {
		t.Fatalf("ExecuteWithConfig: %v", err)
	}
	if !strings.Contains(out, "1+1=2") {
		t.Errorf("expected answer in output, got %q", out)
	}
	if vlm.chatCalls == 0 {
		t.Error("VLM was never called")
	}
	if vlm.shotsSeen == 0 {
		t.Error("VLM did not receive a screenshot")
	}
	if !desk.closed {
		t.Error("desktop was not closed after the run")
	}
}

func TestComputerUseToolMaxStepsReturnsPartial(t *testing.T) {
	// Arrange: VLM never emits done, so the step budget is the only stop.
	vlm := &fakeVLM{resp: `{"action":"mouse_move","coordinate":[0,0]}`}
	tool := newTestComputerUseTool(vlm, &fakeDesktop{})

	// Act
	out, err := tool.ExecuteWithConfig(context.Background(),
		map[string]interface{}{"task": "endless", "max_steps": float64(2)}, nil)

	// Assert: max-steps is a soft partial result, not an error.
	if err != nil {
		t.Fatalf("expected nil error on max-steps, got %v", err)
	}
	if !strings.Contains(out, "未在步数上限内完成") {
		t.Errorf("expected partial-result message, got %q", out)
	}
}
