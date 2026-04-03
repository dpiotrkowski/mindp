package mindp

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

type HLSEvent struct {
	Time      time.Time
	URL       string
	MIMEType  string
	RequestID string
	Kind      string
}

type HLSConfig struct {
	OutputPath       string
	ManifestContains string
	FFmpegArgs       []string
}

type HLSRecorder struct {
	cmd        *exec.Cmd
	manifest   string
	outputPath string
	stderr     *strings.Builder
}

func (r *HLSRecorder) Stop() error {
	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}
	return r.cmd.Process.Kill()
}

func (r *HLSRecorder) Wait() error { return r.cmd.Wait() }

func (r *HLSRecorder) Status() RecorderStatus {
	status := RecorderStatus{ManifestURL: r.manifest, OutputPath: r.outputPath}
	if r.cmd != nil && r.cmd.Process != nil {
		status.PID = r.cmd.Process.Pid
	}
	if r.stderr != nil {
		status.Stderr = r.stderr.String()
	}
	return status
}

type RecorderStatus struct {
	PID         int
	ManifestURL string
	OutputPath  string
	Stderr      string
}

type hlsState struct {
	mu        sync.Mutex
	manifests []string
	handlers  map[int]func(HLSEvent)
	nextID    int
}

func newHLSState() *hlsState {
	return &hlsState{handlers: map[int]func(HLSEvent){}}
}

func (h *hlsState) subscribe(handler func(HLSEvent)) func() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	id := h.nextID
	h.handlers[id] = handler
	return func() {
		h.mu.Lock()
		delete(h.handlers, id)
		h.mu.Unlock()
	}
}

func (h *hlsState) onRequest(raw json.RawMessage) {
	var event struct {
		RequestID string `json:"requestId"`
		Request   struct {
			URL string `json:"url"`
		} `json:"request"`
	}
	if json.Unmarshal(raw, &event) != nil {
		return
	}
	h.ingest(HLSEvent{Time: time.Now(), URL: event.Request.URL, RequestID: event.RequestID, Kind: classifyHLS(event.Request.URL, "")})
}

func (h *hlsState) onResponse(raw json.RawMessage) {
	var event struct {
		RequestID string `json:"requestId"`
		Response  struct {
			URL      string `json:"url"`
			MIMEType string `json:"mimeType"`
		} `json:"response"`
	}
	if json.Unmarshal(raw, &event) != nil {
		return
	}
	h.ingest(HLSEvent{Time: time.Now(), URL: event.Response.URL, MIMEType: event.Response.MIMEType, RequestID: event.RequestID, Kind: classifyHLS(event.Response.URL, event.Response.MIMEType)})
}

func (h *hlsState) ingest(event HLSEvent) {
	if event.Kind == "" {
		return
	}
	h.mu.Lock()
	if event.Kind == "manifest" {
		seen := slices.Contains(h.manifests, event.URL)
		if !seen {
			h.manifests = append(h.manifests, event.URL)
		}
	}
	handlers := make([]func(HLSEvent), 0, len(h.handlers))
	for _, handler := range h.handlers {
		handlers = append(handlers, handler)
	}
	h.mu.Unlock()
	for _, handler := range handlers {
		handler(event)
	}
}

func (h *hlsState) latestManifest(filter string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := len(h.manifests) - 1; i >= 0; i-- {
		if filter == "" || strings.Contains(h.manifests[i], filter) {
			return h.manifests[i]
		}
	}
	return ""
}

func classifyHLS(rawURL, mime string) string {
	if strings.Contains(rawURL, ".m3u8") || strings.Contains(mime, "mpegurl") || strings.Contains(mime, "vnd.apple.mpegurl") {
		return "manifest"
	}
	if strings.Contains(rawURL, ".ts") || strings.Contains(rawURL, ".m4s") || strings.Contains(mime, "mp2t") {
		return "segment"
	}
	return ""
}

func startRecorder(ctx context.Context, manifest string, cfg HLSConfig) (*HLSRecorder, error) {
	if cfg.OutputPath == "" {
		return nil, errors.New("mindp: HLS output path is required")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, err
	}
	args := cfg.FFmpegArgs
	if len(args) == 0 {
		args = []string{"-y", "-i", manifest, "-c", "copy", cfg.OutputPath}
	}
	// #nosec G204 -- ffmpeg is fixed and args are assembled for an explicit recorder workflow.
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr strings.Builder
	cmd.Stdout = nil
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &HLSRecorder{cmd: cmd, manifest: manifest, outputPath: cfg.OutputPath, stderr: &stderr}, nil
}
