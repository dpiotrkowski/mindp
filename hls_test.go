package mindp

import (
	"encoding/json"
	"testing"
)

func TestClassifyHLS(t *testing.T) {
	if got := classifyHLS("https://x.test/live/master.m3u8", "application/vnd.apple.mpegurl"); got != "manifest" {
		t.Fatalf("expected manifest, got %q", got)
	}
	if got := classifyHLS("https://x.test/live/seg-001.ts", "video/mp2t"); got != "segment" {
		t.Fatalf("expected segment, got %q", got)
	}
}

func TestHLSStateIngestAndLatest(t *testing.T) {
	state := newHLSState()
	var seen []HLSEvent
	unsub := state.subscribe(func(event HLSEvent) {
		seen = append(seen, event)
	})
	defer unsub()

	req, _ := json.Marshal(map[string]any{
		"requestId": "1",
		"request":   map[string]any{"url": "https://cdn.test/master.m3u8"},
	})
	resp, _ := json.Marshal(map[string]any{
		"requestId": "2",
		"response":  map[string]any{"url": "https://cdn.test/seg-001.ts", "mimeType": "video/mp2t"},
	})
	state.onRequest(req)
	state.onResponse(resp)

	if len(seen) != 2 {
		t.Fatalf("expected 2 hls events, got %d", len(seen))
	}
	if got := state.latestManifest("master"); got != "https://cdn.test/master.m3u8" {
		t.Fatalf("unexpected latest manifest %q", got)
	}
}
