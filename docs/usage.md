# Usage Guide

## Typical Workflow

The usual `mindp` flow is:

1. launch a browser with `mindp.Launch`
2. create a page with `browser.NewPage`
3. navigate and wait with `WaitLoad` or `WaitReady`
4. interact with the page
5. extract data, persist state, or record HLS if needed
6. close the browser

## Browser Setup

Minimal launch:

```go
browser, err := mindp.Launch(ctx, mindp.Config{
	Headless: true,
})
```

Stealth-oriented launch:

```go
browser, err := mindp.Launch(ctx, mindp.Config{
	Headless: false,
	Persona: &mindp.Persona{
		Locale:         "en-US",
		Languages:      []string{"en-US", "en"},
		Timezone:       "America/New_York",
		WindowSize:     mindp.Size{Width: 1440, Height: 900},
		ScreenSize:     mindp.Size{Width: 1440, Height: 900},
		AcceptLanguage: "en-US,en",
	},
	Stealth: mindp.StealthPolicy{
		Launch: mindp.LaunchProfile{
			Preset: mindp.LaunchPresetStealth,
		},
		Behavior: mindp.BehaviorProfile{
			Mode:       mindp.TimingModeHumanized,
			MinDelay:   40 * time.Millisecond,
			MaxDelay:   120 * time.Millisecond,
			MouseSteps: 8,
		},
	},
})
```

## Waiting and Navigation

Use the waiting primitive that matches the page behavior:

- `WaitLoad`
  - use when a normal load event is a good synchronization point
- `WaitReady`
  - use when you want `document.readyState === "complete"`
- `WaitNetworkIdle`
  - use after client-side boot or async requests

For SPAs or script-heavy pages, `WaitReady` followed by `WaitNetworkIdle` is often the safest baseline.

## Interactions

Deterministic actions:

- `Click`
- `Fill`
- `Press`

Humanized actions:

- `MoveMouse`
- `ClickHuman`
- `FillHuman`
- `TypeHuman`
- `ScrollHuman`
- `Warmup`

Use deterministic actions for stable internal or trusted pages. Use the humanized variants when site behavior or anti-bot heuristics are sensitive to unrealistic input patterns.

## Data Extraction

Common extraction helpers:

- `HTML`
- `Text`
- `Attr`
- `Exists`
- `Count`
- `Eval`

For highly custom data extraction, `Eval` is the escape hatch and should be preferred over expanding the library with site-specific helpers.

## State Persistence

`mindp` supports state capture and replay through:

- `Cookies`
- `SetCookies`
- `State`
- `SaveState`
- `LoadState`

Typical use:

1. log in once
2. save cookies and storage
3. reload that state on later runs

For long-lived trust-building profiles, combine this with a persistent `UserDataDir`.

## Network Controls

Available controls:

- `SetExtraHeaders`
- `BlockURLs`
- `OnRequest`
- `OnResponse`
- browser-side persona-driven UA and language policy

Use these for:

- request observation
- lightweight per-context header shaping
- blocking obvious low-value resources
- debugging site boot flows

## Stealth Inspection

Use `StealthReport` when validating a persona or a launch profile:

```go
report, err := page.StealthReport(ctx)
```

The report is intended as an operator sanity check, not a full stealth certification.

`SaveDebugSnapshot` writes:

- `page.png`
- `page.html`
- `stealth-report.json`

That is the default artifact bundle to capture when a workflow fails unexpectedly.

## HLS Detection and Recording

Typical flow:

```go
stop := page.OnHLS(func(event mindp.HLSEvent) {
	fmt.Println(event.Kind, event.URL)
})
defer stop()

if err := page.Navigate(ctx, targetURL); err != nil { ... }
if err := page.WaitLoad(ctx); err != nil { ... }

recorder, err := page.RecordHLS(ctx, mindp.HLSConfig{
	OutputPath: "capture.ts",
})
if err != nil { ... }
if err := recorder.Wait(); err != nil { ... }
```

`RecordHLS` requires a discovered manifest. If a page uses a different media delivery pattern, `mindp` will not invent a stream where the browser never surfaced one.

## Side-Channel HTTP

`browser.Transport()` returns a persona-bound stdlib transport provider for non-browser HTTP work.

Use it for:

- side-channel API calls
- preflight fetches
- metadata lookups
- browser-adjacent request workflows that should share persona defaults

Do not confuse this with Chromium page traffic. It does not change the browser’s own TLS fingerprint or network stack.
