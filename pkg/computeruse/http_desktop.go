package computeruse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPDesktop controls a remote desktop by calling a desktop sidecar's HTTP API.
// The sidecar (cmd/desktop-sidecar) wraps LocalDesktop (xdotool + scrot) and
// runs next to an Xvfb display inside the desktop container. This lets
// mcp-service drive a desktop that lives in a separate container.
type HTTPDesktop struct {
	baseURL string
	client  *http.Client
}

// NewHTTPDesktop creates a desktop backed by a remote sidecar. baseURL defaults
// to http://desktop:9100 (the docker-compose service name).
func NewHTTPDesktop(baseURL string) *HTTPDesktop {
	if baseURL == "" {
		baseURL = "http://desktop:9100"
	}
	return &HTTPDesktop{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (d *HTTPDesktop) post(ctx context.Context, path string, body interface{}) error {
	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: HTTP %d: %s", path, resp.StatusCode, string(b))
	}
	return nil
}

// Screenshot fetches a PNG from the sidecar.
func (d *HTTPDesktop) Screenshot(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+"/screenshot", nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("screenshot: HTTP %d: %s", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

// ScreenSize queries the display geometry from the sidecar.
func (d *HTTPDesktop) ScreenSize(ctx context.Context) (int, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.baseURL+"/screen_size", nil)
	if err != nil {
		return 0, 0, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("screen_size: HTTP %d", resp.StatusCode)
	}
	var sz struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&sz); err != nil {
		return 0, 0, fmt.Errorf("decode screen_size: %w", err)
	}
	return sz.Width, sz.Height, nil
}

func (d *HTTPDesktop) MouseMove(ctx context.Context, x, y int) error {
	return d.post(ctx, "/mouse_move", map[string]int{"x": x, "y": y})
}

func (d *HTTPDesktop) Click(ctx context.Context, x, y int) error {
	return d.post(ctx, "/click", map[string]int{"x": x, "y": y})
}

func (d *HTTPDesktop) DoubleClick(ctx context.Context, x, y int) error {
	return d.post(ctx, "/double_click", map[string]int{"x": x, "y": y})
}

func (d *HTTPDesktop) RightClick(ctx context.Context, x, y int) error {
	return d.post(ctx, "/right_click", map[string]int{"x": x, "y": y})
}

func (d *HTTPDesktop) Type(ctx context.Context, text string) error {
	return d.post(ctx, "/type", map[string]string{"text": text})
}

func (d *HTTPDesktop) Key(ctx context.Context, combo string) error {
	return d.post(ctx, "/key", map[string]string{"key": combo})
}

func (d *HTTPDesktop) Scroll(ctx context.Context, x, y int, direction string, clicks int) error {
	return d.post(ctx, "/scroll", map[string]interface{}{
		"x":         x,
		"y":         y,
		"direction": direction,
		"clicks":    clicks,
	})
}

func (d *HTTPDesktop) LaunchApp(ctx context.Context, name string) error {
	return d.post(ctx, "/launch_app", map[string]string{"app": name})
}

// Close is a no-op: the desktop lives in the sidecar, not this process.
func (d *HTTPDesktop) Close() error { return nil }
