package mindp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestE2ELoginAndScrape(t *testing.T) {
	if _, err := exec.LookPath("chromium"); err != nil {
		t.Skip("chromium not installed")
	}

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(w, `<!doctype html><html><body>
				<form action="/dashboard" method="get">
					<input id="user" name="user" />
					<input id="pass" name="pass" type="password" />
					<button id="submit" type="submit">Login</button>
				</form>
			</body></html>`)
		case "/dashboard":
			user := r.URL.Query().Get("user")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, `<!doctype html><html><body>
				<div id="welcome">Hello %s</div>
				<div class="value">42</div>
			</body></html>`, user)
		case "/slow":
			time.Sleep(150 * time.Millisecond)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(w, `<!doctype html><html><body><div id="slow">ok</div></body></html>`)
		case "/echo":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, `<!doctype html><html><body><div id="header">%s</div></body></html>`, r.Header.Get("X-Mindp-Test"))
		case "/transport":
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = fmt.Fprintf(w, "%s|%s", r.Header.Get("User-Agent"), r.Header.Get("Accept-Language"))
		case "/block":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, `<!doctype html><html><body><img id="img" src="%s/blocked.png"><div id="done">ready</div></body></html>`, server.URL)
		case "/blocked.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("not-a-real-image"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	browser, err := Launch(ctx, Config{
		Headless: true,
		Persona: &Persona{
			Locale:         "pl-PL",
			Languages:      []string{"pl-PL", "pl"},
			Timezone:       "Europe/Warsaw",
			UserAgent:      "MindpTest/1.0",
			AcceptLanguage: "pl-PL,pl",
			WindowSize:     Size{Width: 1280, Height: 720},
			ScreenSize:     Size{Width: 1280, Height: 720},
		},
		Stealth: StealthPolicy{
			Launch: LaunchProfile{Preset: LaunchPresetStealth},
			Behavior: BehaviorProfile{
				Mode:       TimingModeHumanized,
				MinDelay:   5 * time.Millisecond,
				MaxDelay:   12 * time.Millisecond,
				MouseSteps: 4,
			},
			Transport: TransportProfile{BindPersona: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.NewPage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	runLoginFlow(t, ctx, page, server.URL)
	runNetworkChecks(t, ctx, page, server.URL)
	runStateChecks(t, ctx, page, server.URL)
	assertTransportPersonaBinding(t, ctx, browser, server.URL)
}

func TestE2EHLSDetectAndRecord(t *testing.T) {
	if _, err := exec.LookPath("chromium"); err != nil {
		t.Skip("chromium not installed")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	dir := t.TempDir()
	playlist := filepath.Join(dir, "master.m3u8")
	if err := buildTestHLS(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(playlist); err != nil {
		t.Fatal(err)
	}

	fs := http.FileServer(http.Dir(dir))
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, `<!doctype html><html><body>
				<script>
				fetch(%q).then(r => r.text()).then(x => window.__playlist = x);
				</script>
			</body></html>`, server.URL+"/master.m3u8")
		default:
			fs.ServeHTTP(w, r)
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	browser, err := Launch(ctx, Config{Headless: true})
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.NewPage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	runHLSDetectAndRecord(t, ctx, page, server.URL, dir)
}

func runLoginFlow(t *testing.T, ctx context.Context, page *Page, baseURL string) {
	t.Helper()
	if err := page.Navigate(ctx, baseURL+"/login"); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(ctx); err != nil {
		t.Fatal(err)
	}
	if err := page.Warmup(ctx); err != nil {
		t.Fatal(err)
	}
	if err := page.FillHuman(ctx, "#user", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := page.FillHuman(ctx, "#pass", "secret"); err != nil {
		t.Fatal(err)
	}
	if err := page.ClickHuman(ctx, "#submit"); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(ctx); err != nil {
		t.Fatal(err)
	}
	text, err := page.Text(ctx, "#welcome")
	if err != nil {
		t.Fatal(err)
	}
	if text != "Hello alice" {
		t.Fatalf("unexpected welcome text %q", text)
	}
	html, err := page.HTML(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, `class="value">42</div>`) {
		t.Fatalf("dashboard payload missing expected value: %s", html)
	}
}

func runNetworkChecks(t *testing.T, ctx context.Context, page *Page, baseURL string) {
	t.Helper()
	if err := page.Navigate(ctx, baseURL+"/slow"); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitNetworkIdle(ctx, 250*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	ok, err := page.Exists(ctx, "#slow")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected #slow to exist")
	}
	assertEchoHeaderAndReport(t, ctx, page, baseURL)
	assertObservedEchoRequest(t, ctx, page, baseURL)
	assertBlockedImage(t, ctx, page, baseURL)
}

func assertEchoHeaderAndReport(t *testing.T, ctx context.Context, page *Page, baseURL string) {
	t.Helper()
	if err := page.SetExtraHeaders(ctx, map[string]string{"X-Mindp-Test": "works"}); err != nil {
		t.Fatal(err)
	}
	if err := page.Navigate(ctx, baseURL+"/echo"); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
	headerText, err := page.Text(ctx, "#header")
	if err != nil {
		t.Fatal(err)
	}
	if headerText != "works" {
		t.Fatalf("unexpected echoed header %q", headerText)
	}
	report, err := page.StealthReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Issues) > 0 {
		t.Fatalf("unexpected stealth issues: %v", report.Issues)
	}
	if got, _ := report.Values["language"].(string); got != "pl-PL" {
		t.Fatalf("unexpected reported language %q", got)
	}
}

func assertObservedEchoRequest(t *testing.T, ctx context.Context, page *Page, baseURL string) {
	t.Helper()
	requestSeen := false
	stopRequest := page.OnRequest(func(event RequestEvent) {
		if strings.Contains(event.URL, "/echo") && event.Headers["X-Mindp-Test"] != nil {
			requestSeen = true
		}
	})
	if err := page.Navigate(ctx, baseURL+"/echo"); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
	stopRequest()
	if !requestSeen {
		t.Fatal("expected request observer to see custom header")
	}
}

func assertBlockedImage(t *testing.T, ctx context.Context, page *Page, baseURL string) {
	t.Helper()
	if err := page.BlockURLs(ctx, "*blocked.png*"); err != nil {
		t.Fatal(err)
	}
	if err := page.Navigate(ctx, baseURL+"/block"); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
	rawBlocked, err := page.Eval(ctx, `() => {
		const img = document.querySelector("#img");
		return img ? img.naturalWidth : -1;
	}`)
	if err != nil {
		t.Fatal(err)
	}
	var naturalWidth int
	if err := json.Unmarshal(rawBlocked, &naturalWidth); err != nil {
		t.Fatal(err)
	}
	if naturalWidth != 0 {
		t.Fatalf("expected blocked image naturalWidth 0, got %d", naturalWidth)
	}
}

func runStateChecks(t *testing.T, ctx context.Context, page *Page, baseURL string) {
	t.Helper()
	stateFile := filepath.Join(t.TempDir(), "state.json")
	if _, err := page.Eval(ctx, `() => { localStorage.setItem("token", "abc123"); return true; }`); err != nil {
		t.Fatal(err)
	}
	if err := page.SaveState(ctx, stateFile); err != nil {
		t.Fatal(err)
	}
	if err := page.Navigate(ctx, baseURL+"/echo"); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitReady(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := page.Eval(ctx, `() => { localStorage.clear(); return true; }`); err != nil {
		t.Fatal(err)
	}
	if err := page.LoadState(ctx, stateFile); err != nil {
		t.Fatal(err)
	}
	raw, err := page.Eval(ctx, `() => localStorage.getItem("token")`)
	if err != nil {
		t.Fatal(err)
	}
	var token string
	if err := json.Unmarshal(raw, &token); err != nil {
		t.Fatal(err)
	}
	if token != "abc123" {
		t.Fatalf("unexpected restored token %q", token)
	}
	debugDir := filepath.Join(t.TempDir(), "snapshot")
	if err := page.SaveDebugSnapshot(ctx, debugDir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"page.png", "page.html", "stealth-report.json"} {
		if _, err := os.Stat(filepath.Join(debugDir, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func assertTransportPersonaBinding(t *testing.T, ctx context.Context, browser *Browser, baseURL string) {
	t.Helper()
	resp, err := browser.Transport().Do(ctx, &TransportRequest{URL: baseURL + "/transport"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(resp.Body)); got != "MindpTest/1.0|pl-PL,pl" {
		t.Fatalf("unexpected transport payload %q", got)
	}
}

func runHLSDetectAndRecord(t *testing.T, ctx context.Context, page *Page, baseURL, dir string) {
	t.Helper()
	hlsCh := make(chan HLSEvent, 8)
	stop := page.OnHLS(func(event HLSEvent) {
		hlsCh <- event
	})
	defer stop()
	if err := page.Navigate(ctx, baseURL+"/"); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(ctx); err != nil {
		t.Fatal(err)
	}
	manifest := waitForManifest(t, ctx, hlsCh)
	if !strings.HasSuffix(manifest.URL, "/master.m3u8") {
		t.Fatalf("unexpected manifest url %q", manifest.URL)
	}
	outPath := filepath.Join(dir, "capture.ts")
	recorder, err := page.RecordHLS(ctx, HLSConfig{OutputPath: outPath})
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.Wait(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("expected recorded output")
	}
}

func waitForManifest(t *testing.T, ctx context.Context, hlsCh <-chan HLSEvent) HLSEvent {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			t.Fatal("timeout waiting for HLS detection")
		case event := <-hlsCh:
			if event.Kind == "manifest" {
				return event
			}
		}
	}
}

func buildTestHLS(dir string) error {
	cmd := exec.Command(
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-f", "lavfi",
		"-i", "testsrc=size=160x90:rate=1",
		"-t", "2",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-f", "hls",
		"-hls_time", "1",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(dir, "seg-%03d.ts"),
		filepath.Join(dir, "master.m3u8"),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg build hls failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
