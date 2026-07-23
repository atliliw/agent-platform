// Package main is the desktop sidecar: an HTTP server that wraps
// pkg/computeruse.LocalDesktop (xdotool + scrot) so mcp-service can drive a
// desktop living in a separate container. Run next to an Xvfb display.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"agent-platform/pkg/computeruse"
)

func main() {
	port := os.Getenv("DESKTOP_PORT")
	if port == "" {
		port = "9100"
	}
	if os.Getenv("DISPLAY") == "" {
		log.Println("warning: DISPLAY is empty; xdotool/scrot need an X server (Xvfb)")
	}

	desk := computeruse.NewLocalDesktop()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/screen_size", func(w http.ResponseWriter, r *http.Request) {
		width, height, err := desk.ScreenSize(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]int{"width": width, "height": height})
	})

	mux.HandleFunc("/screenshot", func(w http.ResponseWriter, r *http.Request) {
		img, err := desk.Screenshot(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(img)
	})

	coord := func(fn func(context.Context, int, int) error) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var b struct {
				X int `json:"x"`
				Y int `json:"y"`
			}
			if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
				http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
				return
			}
			if err := fn(r.Context(), b.X, b.Y); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}
	}
	mux.HandleFunc("/mouse_move", coord(desk.MouseMove))
	mux.HandleFunc("/click", coord(desk.Click))
	mux.HandleFunc("/double_click", coord(desk.DoubleClick))
	mux.HandleFunc("/right_click", coord(desk.RightClick))

	mux.HandleFunc("/type", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := desk.Type(r.Context(), b.Text); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/key", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Key string `json:"key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := desk.Key(r.Context(), b.Key); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/scroll", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			X         int    `json:"x"`
			Y         int    `json:"y"`
			Direction string `json:"direction"`
			Clicks    int    `json:"clicks"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := desk.Scroll(r.Context(), b.X, b.Y, b.Direction, b.Clicks); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/launch_app", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			App string `json:"app"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := desk.LaunchApp(r.Context(), b.App); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	addr := ":" + port
	log.Printf("desktop sidecar listening on %s (DISPLAY=%s)", addr, os.Getenv("DISPLAY"))
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("desktop sidecar: %v", err)
	}
}
