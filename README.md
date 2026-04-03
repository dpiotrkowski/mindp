# mindp

`mindp` is a small Chromium CDP automation library written in pure Go stdlib.

It is designed for data-centric browser automation rather than end-to-end testing: logging into sites, extracting structured data, driving real browser flows, detecting HLS streams, and recording browser-discovered media with a minimal dependency footprint.

## Use Cases

- authenticated scraping and data extraction
- login flows and browser-controlled form automation
- Chromium CDP scripting in Go without a large framework
- stealth-aware browser automation for data collection
- HLS manifest detection and `ffmpeg` recording
- side-channel HTTP work that stays aligned with the active browser persona

## Design Goals

- zero external Go module dependencies
- Chromium-first automation with clean, direct APIs
- headful and headless local launch
- persona-driven stealth with a coherent five-layer model
- practical browser control for scraping and authenticated data collection
- built-in HLS detection and managed `ffmpeg` recording

## What `mindp` Is

`mindp` is intentionally narrow:

- a small CDP client and browser runtime you can own
- a lightweight automation layer over the specific CDP methods needed for common data workflows
- a stealth-aware runtime with observable behavior and explicit tradeoffs

It is not:

- a generated full-CDP SDK
- a cross-browser abstraction layer
- a testing framework
- a patched anti-detect browser

## Requirements

- Go 1.26+
- local Chromium-compatible browser installed as `chromium`, `google-chrome`, or `chromium-browser`, unless `ExecutablePath` is set explicitly
- `ffmpeg` on `PATH` only if HLS recording is used

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"mindp"
)

func main() {
	ctx := context.Background()

	browser, err := mindp.Launch(ctx, mindp.Config{
		Headless: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.NewPage(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if err := page.Navigate(ctx, "https://example.com"); err != nil {
		log.Fatal(err)
	}
	if err := page.WaitReady(ctx); err != nil {
		log.Fatal(err)
	}

	html, err := page.HTML(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(html))
}
```

## Main APIs

Core browser lifecycle:

- `mindp.Launch(ctx, cfg)`
- `(*Browser).NewPage(ctx)`
- `(*Browser).Close()`
- `(*Browser).Persona()`
- `(*Browser).Transport()`

Page automation:

- `Navigate`, `WaitLoad`, `WaitReady`, `WaitNetworkIdle`
- `Eval`, `HTML`, `Text`, `Exists`, `Count`, `Attr`
- `Click`, `Fill`, `Press`
- `ClickHuman`, `FillHuman`, `TypeHuman`, `MoveMouse`, `ScrollHuman`, `Warmup`
- `Cookies`, `SetCookies`, `State`, `SaveState`, `LoadState`
- `SetExtraHeaders`, `BlockURLs`, `OnRequest`, `OnResponse`
- `Screenshot`, `SaveDebugSnapshot`, `StealthReport`

Media:

- `OnHLS`
- `RecordHLS`

Examples:

- [examples/login_scrape/main.go](/home/admin/mindp/examples/login_scrape/main.go)
- [examples/hls_record/main.go](/home/admin/mindp/examples/hls_record/main.go)

## Persona and Stealth

`mindp` drives stealth through a single persona and five coordinated layers:

1. `surface`
2. `launch`
3. `network`
4. `transport`
5. `behavior`

The important design point is coherence. The browser runtime, launch flags, locale, timezone, client hints, headers, side-channel HTTP behavior, and humanized actions should describe the same machine and user profile.

Key entry points:

- `Config.Persona`
- `Config.Stealth`
- `page.StealthReport(ctx)`
- `browser.Transport()`

For hostile targets, use:

- headful mode where practical
- persistent profiles where trust accumulation matters
- aligned proxy, locale, timezone, and language settings
- `StealthReport` and canary checks before scaling traffic

## Configuration Example

```go
browser, err := mindp.Launch(ctx, mindp.Config{
	Headless: false,
	Persona: &mindp.Persona{
		Locale:         "en-US",
		Languages:      []string{"en-US", "en"},
		Timezone:       "America/New_York",
		UserAgent:      "Mozilla/5.0 ...",
		AcceptLanguage: "en-US,en",
		WindowSize:     mindp.Size{Width: 1440, Height: 900},
		ScreenSize:     mindp.Size{Width: 1440, Height: 900},
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
		Network: mindp.NetworkPolicy{
			Headers: map[string]string{
				"Accept": "text/html,application/xhtml+xml",
			},
		},
	},
})
```

## HLS Recording

`mindp` can detect HLS manifests from browser traffic and start a managed `ffmpeg` process once a manifest is seen.

Typical flow:

1. Navigate to the page that loads the media.
2. Subscribe with `page.OnHLS(...)` or wait for `RecordHLS(...)` to find a manifest.
3. Start recording with `page.RecordHLS(ctx, cfg)`.
4. Wait for the recorder or stop it explicitly.

This is designed for browser-discovered streams. It is not a full media pipeline or transcription engine.

## Current Scope

Supported well:

- local Chromium launch
- remote CDP seam via `ProviderConfig`
- browser automation for forms, navigation, state reuse, and data extraction
- HLS detection and `ffmpeg` recording
- side-channel stdlib HTTP transport bound to the active persona

Not solved natively:

- patched-browser stealth comparable to Patchright or Camoufox
- TLS/JA3 impersonation for Chromium page traffic
- CAPTCHA solving
- full anti-bot bypass guarantees

## Documentation

- [Architecture](/home/admin/mindp/docs/architecture.md)
- [Usage Guide](/home/admin/mindp/docs/usage.md)
- [Stealth Research](/home/admin/mindp/docs/stealth-research.md)
- [Stealth Threat Model](/home/admin/mindp/docs/stealth-threat-model.md)
- [Provider Evaluation](/home/admin/mindp/docs/provider-evaluation.md)

## Quality Status

The current tree passes:

- `go fix ./...`
- `gofumpt -w .`
- `staticcheck ./...`
- `ineffassign ./...`
- `unparam ./...`
- `gocyclo -over 15 .`
- `gosec ./...`
- `govulncheck ./...`
- `go vet ./...`
- `go test ./...`
- `go test -race ./...`
