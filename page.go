package mindp

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Page struct {
	browser   *Browser
	targetID  string
	sessionID string

	hlsState *hlsState

	loadMu     sync.Mutex
	loadWaiter chan struct{}
	reqMu      sync.Mutex
	reqWaiter  chan struct{}
	mouseMu    sync.Mutex
	mouseX     float64
	mouseY     float64
	mouseSet   bool
}

func (p *Page) enable(ctx context.Context) error {
	if err := p.browser.conn.call(ctx, p.sessionID, "Page.enable", nil, nil); err != nil {
		return err
	}
	if err := p.browser.conn.call(ctx, p.sessionID, "Runtime.enable", nil, nil); err != nil {
		return err
	}
	if err := p.browser.conn.call(ctx, p.sessionID, "Network.enable", nil, nil); err != nil {
		return err
	}
	if err := p.applyPersona(ctx); err != nil {
		return err
	}
	if !p.browser.cfg.DisableStealth {
		source := stealthSource(p.browser.cfg)
		if err := p.browser.conn.call(ctx, p.sessionID, "Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": source}, &addScriptResult{}); err != nil {
			return err
		}
		_ = p.browser.conn.call(ctx, p.sessionID, "Runtime.evaluate", map[string]any{"expression": source}, nil)
	}
	p.loadWaiter = make(chan struct{}, 1)
	p.reqWaiter = make(chan struct{}, 32)
	p.browser.conn.subscribe(p.sessionID, "Page.loadEventFired", func(_ json.RawMessage) {
		p.loadMu.Lock()
		select {
		case p.loadWaiter <- struct{}{}:
		default:
		}
		p.loadMu.Unlock()
	})
	p.browser.conn.subscribe(p.sessionID, "Network.requestWillBeSent", func(raw json.RawMessage) {
		p.reqMu.Lock()
		select {
		case p.reqWaiter <- struct{}{}:
		default:
		}
		p.reqMu.Unlock()
		p.hlsState.onRequest(raw)
	})
	p.browser.conn.subscribe(p.sessionID, "Network.responseReceived", p.hlsState.onResponse)
	return nil
}

func (p *Page) applyPersona(ctx context.Context) error {
	persona := p.browser.persona
	if persona.Timezone != "" {
		_ = p.Call(ctx, "Emulation.setTimezoneOverride", map[string]any{"timezoneId": persona.Timezone}, nil)
	}
	if persona.Locale != "" {
		_ = p.Call(ctx, "Emulation.setLocaleOverride", map[string]any{"locale": persona.Locale}, nil)
	}
	if persona.WindowSize.Width > 0 && persona.WindowSize.Height > 0 {
		_ = p.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{
			"width":             persona.WindowSize.Width,
			"height":            persona.WindowSize.Height,
			"deviceScaleFactor": 1,
			"mobile":            false,
			"screenWidth":       persona.ScreenSize.Width,
			"screenHeight":      persona.ScreenSize.Height,
		}, nil)
	}
	params := map[string]any{
		"userAgent":      effectiveUserAgent(p.browser.cfg, persona),
		"acceptLanguage": effectiveAcceptLanguage(p.browser.cfg, persona),
		"platform":       persona.Platform,
	}
	if metadata := userAgentMetadata(p.browser.cfg, persona); len(metadata) > 0 {
		params["userAgentMetadata"] = metadata
	}
	if err := p.Call(ctx, "Network.setUserAgentOverride", params, nil); err != nil {
		return err
	}
	return p.ApplyNetworkPolicy(ctx)
}

func (p *Page) Call(ctx context.Context, method string, params any, result any) error {
	return p.browser.conn.call(ctx, p.sessionID, method, params, result)
}

func (p *Page) OnEvent(method string, handler func(json.RawMessage)) func() {
	return p.browser.conn.subscribe(p.sessionID, method, handler)
}

func (p *Page) Navigate(ctx context.Context, rawURL string) error {
	return p.Call(ctx, "Page.navigate", map[string]any{"url": rawURL}, nil)
}

func (p *Page) WaitReady(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, err := p.Eval(ctx, `() => document.readyState`)
		if err == nil {
			var state string
			if json.Unmarshal(raw, &state) == nil && state == "complete" {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (p *Page) WaitNetworkIdle(ctx context.Context, idleFor time.Duration) error {
	if idleFor <= 0 {
		idleFor = 500 * time.Millisecond
	}
	timer := time.NewTimer(idleFor)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.reqWaiter:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(idleFor)
		case <-timer.C:
			return nil
		}
	}
}

func (p *Page) WaitLoad(ctx context.Context) error {
	p.loadMu.Lock()
	waiter := p.loadWaiter
	p.loadMu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-waiter:
		return nil
	}
}

func (p *Page) Eval(ctx context.Context, js string, args ...any) (json.RawMessage, error) {
	expr := buildExpression(js, args)
	var out evaluateResult
	if err := p.Call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    expr,
		"returnByValue": true,
		"awaitPromise":  true,
	}, &out); err != nil {
		return nil, err
	}
	return out.Result.Value, nil
}

func (p *Page) HTML(ctx context.Context) (string, error) {
	raw, err := p.Eval(ctx, "() => document.documentElement.outerHTML")
	if err != nil {
		return "", err
	}
	return decodeString(raw)
}

func (p *Page) Text(ctx context.Context, selector string) (string, error) {
	raw, err := p.Eval(ctx, "(sel) => { const el = document.querySelector(sel); return el ? (el.textContent || '') : ''; }", selector)
	if err != nil {
		return "", err
	}
	return decodeString(raw)
}

func (p *Page) Exists(ctx context.Context, selector string) (bool, error) {
	raw, err := p.Eval(ctx, "(sel) => !!document.querySelector(sel)", selector)
	if err != nil {
		return false, err
	}
	var ok bool
	if err := json.Unmarshal(raw, &ok); err != nil {
		return false, err
	}
	return ok, nil
}

func (p *Page) Count(ctx context.Context, selector string) (int, error) {
	raw, err := p.Eval(ctx, "(sel) => document.querySelectorAll(sel).length", selector)
	if err != nil {
		return 0, err
	}
	var count int
	if err := json.Unmarshal(raw, &count); err != nil {
		return 0, err
	}
	return count, nil
}

func (p *Page) Attr(ctx context.Context, selector, name string) (string, error) {
	raw, err := p.Eval(ctx, "(sel, name) => { const el = document.querySelector(sel); return el ? (el.getAttribute(name) || '') : ''; }", selector, name)
	if err != nil {
		return "", err
	}
	return decodeString(raw)
}

func (p *Page) Click(ctx context.Context, selector string) error {
	x, y, err := p.elementCenter(ctx, selector)
	if err != nil {
		return err
	}
	if err := p.Call(ctx, "Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x, "y": y, "button": "none"}, nil); err != nil {
		return err
	}
	if err := p.Call(ctx, "Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": x, "y": y, "button": "left", "clickCount": 1}, nil); err != nil {
		return err
	}
	return p.Call(ctx, "Input.dispatchMouseEvent", map[string]any{"type": "mouseReleased", "x": x, "y": y, "button": "left", "clickCount": 1}, nil)
}

func (p *Page) Fill(ctx context.Context, selector, value string) error {
	if err := p.Click(ctx, selector); err != nil {
		return err
	}
	_, err := p.Eval(ctx, `(sel) => {
		const el = document.querySelector(sel);
		if (!el) throw new Error("element not found");
		el.focus();
		if ('value' in el) {
			el.value = '';
			el.dispatchEvent(new Event("input", { bubbles: true }));
		}
		return true;
	}`, selector)
	if err != nil {
		return err
	}
	return p.Call(ctx, "Input.insertText", map[string]any{"text": value}, nil)
}

func (p *Page) Press(ctx context.Context, key string) error {
	if err := p.Call(ctx, "Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": key, "text": key}, nil); err != nil {
		return err
	}
	return p.Call(ctx, "Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": key}, nil)
}

func (p *Page) MoveMouse(ctx context.Context, x, y float64) error {
	startX, startY := p.mousePosition()
	steps := max(p.browser.cfg.Stealth.Behavior.MouseSteps, 1)
	for i := 1; i <= steps; i++ {
		nx := startX + (x-startX)*(float64(i)/float64(steps))
		ny := startY + (y-startY)*(float64(i)/float64(steps))
		if err := p.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
			"type":   "mouseMoved",
			"x":      math.Round(nx*100) / 100,
			"y":      math.Round(ny*100) / 100,
			"button": "none",
		}, nil); err != nil {
			return err
		}
		p.setMousePosition(nx, ny)
		if err := p.behaviorPause(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *Page) ClickHuman(ctx context.Context, selector string) error {
	x, y, err := p.elementCenter(ctx, selector)
	if err != nil {
		return err
	}
	if err := p.MoveMouse(ctx, x, y); err != nil {
		return err
	}
	if err := p.behaviorPause(ctx); err != nil {
		return err
	}
	if err := p.Call(ctx, "Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": x, "y": y, "button": "left", "clickCount": 1}, nil); err != nil {
		return err
	}
	if err := p.behaviorPause(ctx); err != nil {
		return err
	}
	if err := p.Call(ctx, "Input.dispatchMouseEvent", map[string]any{"type": "mouseReleased", "x": x, "y": y, "button": "left", "clickCount": 1}, nil); err != nil {
		return err
	}
	p.setMousePosition(x, y)
	return nil
}

func (p *Page) TypeHuman(ctx context.Context, value string) error {
	for _, r := range value {
		if err := p.Call(ctx, "Input.insertText", map[string]any{"text": string(r)}, nil); err != nil {
			return err
		}
		if err := p.behaviorPause(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *Page) FillHuman(ctx context.Context, selector, value string) error {
	if err := p.ClickHuman(ctx, selector); err != nil {
		return err
	}
	_, err := p.Eval(ctx, `(sel) => {
		const el = document.querySelector(sel);
		if (!el) throw new Error("element not found");
		el.focus();
		if ('value' in el) {
			el.value = '';
			el.dispatchEvent(new Event("input", { bubbles: true }));
		}
		return true;
	}`, selector)
	if err != nil {
		return err
	}
	return p.TypeHuman(ctx, value)
}

func (p *Page) ScrollHuman(ctx context.Context, deltaY float64) error {
	x, y := p.mousePosition()
	if err := p.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type":        "mouseWheel",
		"x":           x,
		"y":           y,
		"deltaY":      deltaY,
		"deltaX":      0,
		"button":      "none",
		"pointerType": "mouse",
	}, nil); err != nil {
		return err
	}
	return p.behaviorPause(ctx)
}

func (p *Page) Warmup(ctx context.Context) error {
	if err := p.WaitReady(ctx); err != nil {
		return err
	}
	if err := p.WaitNetworkIdle(ctx, 300*time.Millisecond); err != nil {
		return err
	}
	if err := sleepContext(ctx, p.browser.cfg.Stealth.Behavior.WarmupDelay); err != nil {
		return err
	}
	if err := p.ScrollHuman(ctx, 180); err != nil {
		return err
	}
	if err := p.ScrollHuman(ctx, -180); err != nil {
		return err
	}
	return nil
}

func (p *Page) WaitVisible(ctx context.Context, selector string) error {
	return p.waitCondition(ctx, `(sel) => {
		const el = document.querySelector(sel);
		if (!el) return false;
		const rect = el.getBoundingClientRect();
		const style = window.getComputedStyle(el);
		return rect.width > 0 && rect.height > 0 && style.visibility !== "hidden" && style.display !== "none";
	}`, selector)
}

func (p *Page) WaitGone(ctx context.Context, selector string) error {
	return p.waitCondition(ctx, `(sel) => !document.querySelector(sel)`, selector)
}

func (p *Page) Screenshot(ctx context.Context, path string) error {
	var out screenshotResult
	if err := p.Call(ctx, "Page.captureScreenshot", map[string]any{"format": "png", "captureBeyondViewport": true}, &out); err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(out.Data)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (p *Page) Cookies(ctx context.Context) ([]Cookie, error) {
	var out struct {
		Cookies []Cookie `json:"cookies"`
	}
	if err := p.Call(ctx, "Network.getCookies", nil, &out); err != nil {
		return nil, err
	}
	return out.Cookies, nil
}

func (p *Page) SetCookies(ctx context.Context, cookies ...Cookie) error {
	params := map[string]any{"cookies": cookies}
	return p.Call(ctx, "Network.setCookies", params, nil)
}

func (p *Page) SetExtraHeaders(ctx context.Context, headers map[string]string) error {
	return p.Call(ctx, "Network.setExtraHTTPHeaders", map[string]any{"headers": headers}, nil)
}

func (p *Page) ApplyNetworkPolicy(ctx context.Context) error {
	headers := make(map[string]string)
	maps.Copy(headers, p.browser.cfg.Stealth.Network.Headers)
	if accept := effectiveAcceptLanguage(p.browser.cfg, p.browser.persona); accept != "" {
		headers["Accept-Language"] = accept
	}
	if len(headers) > 0 {
		if err := p.SetExtraHeaders(ctx, headers); err != nil {
			return err
		}
	}
	if len(p.browser.cfg.Stealth.Network.BlockedURLs) > 0 {
		if err := p.BlockURLs(ctx, p.browser.cfg.Stealth.Network.BlockedURLs...); err != nil {
			return err
		}
	}
	return nil
}

func (p *Page) BlockURLs(ctx context.Context, patterns ...string) error {
	return p.Call(ctx, "Network.setBlockedURLs", map[string]any{"urls": patterns}, nil)
}

func (p *Page) OnRequest(handler func(RequestEvent)) func() {
	return p.OnEvent("Network.requestWillBeSent", func(raw json.RawMessage) {
		var event struct {
			RequestID string         `json:"requestId"`
			Timestamp float64        `json:"timestamp"`
			Request   RequestEventJS `json:"request"`
		}
		if json.Unmarshal(raw, &event) != nil {
			return
		}
		handler(RequestEvent{
			Time:      int64(event.Timestamp * 1000),
			RequestID: event.RequestID,
			URL:       event.Request.URL,
			Method:    event.Request.Method,
			Headers:   event.Request.Headers,
		})
	})
}

func (p *Page) OnResponse(handler func(ResponseEvent)) func() {
	return p.OnEvent("Network.responseReceived", func(raw json.RawMessage) {
		var event struct {
			RequestID string  `json:"requestId"`
			Timestamp float64 `json:"timestamp"`
			Response  struct {
				URL      string         `json:"url"`
				Status   int            `json:"status"`
				MIMEType string         `json:"mimeType"`
				Headers  map[string]any `json:"headers"`
			} `json:"response"`
		}
		if json.Unmarshal(raw, &event) != nil {
			return
		}
		handler(ResponseEvent{
			Time:      int64(event.Timestamp * 1000),
			RequestID: event.RequestID,
			URL:       event.Response.URL,
			Status:    event.Response.Status,
			MIMEType:  event.Response.MIMEType,
			Headers:   event.Response.Headers,
		})
	})
}

func (p *Page) SaveState(ctx context.Context, path string) error {
	state, err := p.State(ctx)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (p *Page) LoadState(ctx context.Context, path string) error {
	// #nosec G304 -- state loading is an explicit caller-controlled file operation.
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var state storageState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	if len(state.Cookies) > 0 {
		if err := p.SetCookies(ctx, state.Cookies...); err != nil {
			return err
		}
	}
	if state.Origin != "" {
		if err := p.Navigate(ctx, state.Origin); err != nil {
			return err
		}
		if err := p.WaitReady(ctx); err != nil {
			return err
		}
	}
	if len(state.LocalStorage) > 0 || len(state.SessionStore) > 0 {
		if _, err := p.Eval(ctx, `(localState, sessionState) => {
			for (const [k,v] of Object.entries(localState || {})) localStorage.setItem(k, v);
			for (const [k,v] of Object.entries(sessionState || {})) sessionStorage.setItem(k, v);
			return true;
		}`, state.LocalStorage, state.SessionStore); err != nil {
			return err
		}
	}
	return nil
}

func (p *Page) State(ctx context.Context) (*storageState, error) {
	cookies, err := p.Cookies(ctx)
	if err != nil {
		return nil, err
	}
	raw, err := p.Eval(ctx, `() => ({
		origin: location.origin,
		localStorage: Object.fromEntries(Object.entries(localStorage)),
		sessionStorage: Object.fromEntries(Object.entries(sessionStorage)),
	})`)
	if err != nil {
		return nil, err
	}
	var state storageState
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	state.Cookies = cookies
	return &state, nil
}

func (p *Page) SaveDebugSnapshot(ctx context.Context, dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	if err := p.Screenshot(ctx, filepath.Join(dir, "page.png")); err != nil {
		return err
	}
	html, err := p.HTML(ctx)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "page.html"), []byte(html), 0o600); err != nil {
		return err
	}
	report, err := p.StealthReport(ctx)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "stealth-report.json"), data, 0o600)
}

func (p *Page) OnHLS(handler func(HLSEvent)) func() {
	return p.hlsState.subscribe(handler)
}

func (p *Page) RecordHLS(ctx context.Context, cfg HLSConfig) (*HLSRecorder, error) {
	manifest := p.hlsState.latestManifest(cfg.ManifestContains)
	if manifest == "" {
		return nil, ErrNoHLSManifest
	}
	return startRecorder(ctx, manifest, cfg)
}

func (p *Page) StealthReport(ctx context.Context) (*StealthReport, error) {
	raw, err := p.Eval(ctx, `() => ({
		webdriver: navigator.webdriver,
		language: navigator.language,
		languages: navigator.languages,
		platform: navigator.platform,
		userAgent: navigator.userAgent,
		hardwareConcurrency: navigator.hardwareConcurrency,
		deviceMemory: navigator.deviceMemory,
		timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
		colorSchemeDark: window.matchMedia('(prefers-color-scheme: dark)').matches,
		reducedMotion: window.matchMedia('(prefers-reduced-motion: reduce)').matches,
		screen: {
			width: screen.width,
			height: screen.height,
			availWidth: screen.availWidth,
			availHeight: screen.availHeight,
		},
	})`)
	if err != nil {
		return nil, err
	}
	values := make(map[string]any)
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	report := &StealthReport{Persona: p.browser.persona, Values: values}
	if wd, ok := values["webdriver"]; ok && wd != nil {
		report.Issues = append(report.Issues, "navigator.webdriver is exposed")
	}
	if tz, _ := values["timezone"].(string); p.browser.persona.Timezone != "" && tz != p.browser.persona.Timezone {
		report.Issues = append(report.Issues, "timezone does not match persona")
	}
	if lang, _ := values["language"].(string); p.browser.persona.Locale != "" && lang != p.browser.persona.Locale {
		report.Issues = append(report.Issues, "navigator.language does not match persona")
	}
	return report, nil
}

func buildExpression(js string, args []any) string {
	if len(args) == 0 {
		return "(" + js + ")()"
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		data, _ := json.Marshal(arg)
		parts = append(parts, string(data))
	}
	return fmt.Sprintf("(%s)(%s)", js, strings.Join(parts, ","))
}

func decodeString(raw json.RawMessage) (string, error) {
	var out string
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out, nil
}

func (p *Page) waitCondition(ctx context.Context, js string, selector string) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		raw, err := p.Eval(ctx, js, selector)
		if err == nil {
			var ok bool
			if json.Unmarshal(raw, &ok) == nil && ok {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (p *Page) elementCenter(ctx context.Context, selector string) (float64, float64, error) {
	var doc nodeResult
	if err := p.Call(ctx, "DOM.getDocument", map[string]any{"depth": 1}, &doc); err != nil {
		return 0, 0, err
	}
	var node querySelectorResult
	if err := p.Call(ctx, "DOM.querySelector", map[string]any{"nodeId": doc.Root.NodeID, "selector": selector}, &node); err != nil {
		return 0, 0, err
	}
	if node.NodeID == 0 {
		return 0, 0, errElementNotFound
	}
	_ = p.Call(ctx, "DOM.scrollIntoViewIfNeeded", map[string]any{"nodeId": node.NodeID}, nil)
	var box domBoxModel
	if err := p.Call(ctx, "DOM.getBoxModel", map[string]any{"nodeId": node.NodeID}, &box); err != nil {
		return 0, 0, err
	}
	if len(box.Model.Content) < 8 {
		return 0, 0, errors.New("mindp: invalid box model")
	}
	return (box.Model.Content[0] + box.Model.Content[4]) / 2, (box.Model.Content[1] + box.Model.Content[5]) / 2, nil
}

func (p *Page) behaviorPause(ctx context.Context) error {
	mode := p.browser.cfg.Stealth.Behavior.Mode
	if mode == TimingModeDeterministic {
		return nil
	}
	minDelay := p.browser.cfg.Stealth.Behavior.MinDelay
	maxDelay := max(p.browser.cfg.Stealth.Behavior.MaxDelay, minDelay)
	delay := minDelay
	if mode == TimingModeHumanized || mode == TimingModeJittered {
		if span := maxDelay - minDelay; span > 0 {
			delay += time.Duration(p.randInt63n(int64(span)))
		}
	}
	return sleepContext(ctx, delay)
}

func (p *Page) randInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	v, err := cryptorand.Int(cryptorand.Reader, big.NewInt(n))
	if err != nil {
		return 0
	}
	return v.Int64()
}

func (p *Page) mousePosition() (float64, float64) {
	p.mouseMu.Lock()
	defer p.mouseMu.Unlock()
	if p.mouseSet {
		return p.mouseX, p.mouseY
	}
	return 16, 16
}

func (p *Page) setMousePosition(x, y float64) {
	p.mouseMu.Lock()
	p.mouseX = x
	p.mouseY = y
	p.mouseSet = true
	p.mouseMu.Unlock()
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type Cookie = cookie

var errElementNotFound = errors.New("mindp: element not found")
