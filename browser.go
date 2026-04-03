package mindp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Browser struct {
	cfg            Config
	persona        Persona
	conn           *conn
	cmd            *exec.Cmd
	debugURL       string
	userDataDir    string
	cleanupProfile bool
	transport      TransportProvider

	mu    sync.Mutex
	pages map[string]*Page
}

func Launch(ctx context.Context, cfg Config) (*Browser, error) {
	cfg = cfg.withDefaults()
	userDataDir, cleanup, err := profileDir(cfg)
	if err != nil {
		return nil, err
	}
	startCtx, cancel := context.WithTimeout(ctx, cfg.Timeouts.Startup)
	defer cancel()
	cmd, debugURL, browserVersion, err := connectBrowser(ctx, startCtx, cfg, userDataDir)
	if err != nil {
		if cleanup {
			_ = os.RemoveAll(userDataDir)
		}
		return nil, err
	}
	persona := resolvePersona(cfg, userDataDir, browserVersion)
	conn, err := newConn(startCtx, debugURL)
	if err != nil {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		if cleanup {
			_ = os.RemoveAll(userDataDir)
		}
		return nil, err
	}
	cfg.Persona = &persona
	b := &Browser{
		cfg:            cfg,
		persona:        persona,
		conn:           conn,
		cmd:            cmd,
		debugURL:       debugURL,
		userDataDir:    userDataDir,
		cleanupProfile: cleanup,
		transport:      newStdlibTransport(cfg, persona),
		pages:          map[string]*Page{},
	}
	return b, nil
}

func (b *Browser) NewPage(ctx context.Context) (*Page, error) {
	var target createTargetResult
	if err := b.conn.call(ctx, "", "Target.createTarget", map[string]any{"url": "about:blank"}, &target); err != nil {
		return nil, err
	}
	var attached attachTargetResult
	if err := b.conn.call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": target.TargetID, "flatten": true}, &attached); err != nil {
		return nil, err
	}
	page := &Page{
		browser:   b,
		targetID:  target.TargetID,
		sessionID: attached.SessionID,
		hlsState:  newHLSState(),
	}
	if err := page.enable(ctx); err != nil {
		return nil, err
	}
	b.mu.Lock()
	b.pages[target.TargetID] = page
	b.mu.Unlock()
	return page, nil
}

func (b *Browser) Pages(ctx context.Context) ([]*Page, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]*Page, 0, len(b.pages))
	for _, p := range b.pages {
		out = append(out, p)
	}
	return out, nil
}

func (b *Browser) Close() error {
	if b.conn != nil {
		_ = b.conn.close()
	}
	if b.cmd != nil && b.cmd.Process != nil {
		_ = b.cmd.Process.Kill()
		_, _ = b.cmd.Process.Wait()
	}
	if b.cleanupProfile {
		_ = os.RemoveAll(b.userDataDir)
	}
	return nil
}

func (b *Browser) Call(ctx context.Context, method string, params any, result any) error {
	return b.conn.call(ctx, "", method, params, result)
}

func (b *Browser) OnEvent(method string, handler func(json.RawMessage)) func() {
	return b.conn.subscribe("", method, handler)
}

func (b *Browser) UserDataDir() string {
	return b.userDataDir
}

func (b *Browser) Persona() Persona {
	return b.persona
}

func (b *Browser) DebugURL() string {
	return b.debugURL
}

func (b *Browser) Transport() TransportProvider {
	return b.transport
}

func chromiumArgs(cfg Config, port int, userDataDir string) []string {
	locale := "en-US"
	if cfg.Persona != nil && cfg.Persona.Locale != "" {
		locale = cfg.Persona.Locale
	}
	args := []string{
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--no-first-run",
		"--no-default-browser-check",
		"--user-data-dir=" + userDataDir,
		fmt.Sprintf("--window-size=%d,%d", cfg.WindowSize.Width, cfg.WindowSize.Height),
		"--lang=" + locale,
	}
	args = append(args, presetArgs(cfg.Stealth.Launch.Preset)...)
	if cfg.Headless {
		args = append(args, "--headless=new")
	}
	if cfg.Proxy != "" {
		args = append(args, "--proxy-server="+cfg.Proxy)
	}
	args = append(args, cfg.Args...)
	return args
}

func resolveExecutable(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	candidates := []string{"chromium", "google-chrome", "chromium-browser"}
	for _, name := range candidates {
		path, err := exec.LookPath(name)
		if err == nil {
			return path, nil
		}
	}
	return "", errors.New("mindp: chromium executable not found")
}

func profileDir(cfg Config) (string, bool, error) {
	if cfg.UserDataDir != "" {
		return cfg.UserDataDir, false, nil
	}
	dir, err := os.MkdirTemp("", "mindp-profile-*")
	if err != nil {
		return "", false, err
	}
	return dir, true, nil
}

func connectBrowser(cmdCtx, startupCtx context.Context, cfg Config, userDataDir string) (*exec.Cmd, string, string, error) {
	if cfg.Provider.Kind == ProviderKindRemoteCDP && cfg.Provider.DebugURL != "" {
		return nil, cfg.Provider.DebugURL, "", nil
	}
	exe, err := resolveExecutable(cfg.ExecutablePath)
	if err != nil {
		return nil, "", "", err
	}
	port, err := freePort()
	if err != nil {
		return nil, "", "", err
	}
	args := chromiumArgs(cfg, port, userDataDir)
	// #nosec G204 -- executable path is resolved locally and arguments are generated Chromium flags.
	cmd := exec.CommandContext(cmdCtx, exe, args...)
	cmd.Env = append(os.Environ(), cfg.Env...)
	var stderr bytes.Buffer
	cmd.Stdout = ioDiscard{}
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, "", "", err
	}
	info, err := waitVersionInfo(startupCtx, port)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, "", "", fmt.Errorf("mindp: chromium startup failed: %w stderr=%s", err, strings.TrimSpace(stderr.String()))
	}
	return cmd, info.WebSocketDebuggerURL, chromeVersion(info.Browser), nil
}

func chromeVersion(browser string) string {
	if idx := strings.LastIndex(browser, "/"); idx >= 0 && idx+1 < len(browser) {
		return browser[idx+1:]
	}
	return ""
}

func presetArgs(preset LaunchPreset) []string {
	switch preset {
	case LaunchPresetDebug:
		return []string{"--disable-background-networking"}
	case LaunchPresetStealth:
		return []string{
			"--disable-background-networking",
			"--disable-background-timer-throttling",
			"--disable-backgrounding-occluded-windows",
			"--disable-renderer-backgrounding",
			"--disable-features=Translate,MediaRouter",
			"--disable-blink-features=AutomationControlled",
		}
	case LaunchPresetHostile:
		return []string{
			"--disable-background-networking",
			"--disable-background-timer-throttling",
			"--disable-backgrounding-occluded-windows",
			"--disable-renderer-backgrounding",
			"--disable-features=Translate,MediaRouter,OptimizationHints",
			"--disable-blink-features=AutomationControlled",
			"--password-store=basic",
		}
	default:
		return []string{
			"--disable-background-networking",
			"--disable-background-timer-throttling",
			"--disable-backgrounding-occluded-windows",
			"--disable-renderer-backgrounding",
			"--disable-features=AutomationControlled,Translate,MediaRouter",
		}
	}
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func waitVersionInfo(ctx context.Context, port int) (*wsVersionInfo, error) {
	client := &http.Client{Timeout: time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			var info wsVersionInfo
			err = json.NewDecoder(resp.Body).Decode(&info)
			_ = resp.Body.Close()
			if err == nil && info.WebSocketDebuggerURL != "" {
				return &info, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
