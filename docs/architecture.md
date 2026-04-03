# Architecture

## Overview

`mindp` is built as a small, opinionated Chromium automation runtime.

The implementation keeps the stack intentionally narrow:

- a minimal in-repo WebSocket client
- a generic CDP request and event router
- a browser/page API focused on real automation workflows
- a persona-driven stealth model shared across multiple runtime layers
- HLS discovery and managed recording support

The library does not generate or vendor a full CDP schema. It handwrites only the protocol methods and structures needed for the supported flows.

## Layers

### Transport and CDP

- `internal/ws`
  - minimal RFC6455 client
  - stdlib only
  - enough for Chromium DevTools transport without pulling a larger client dependency
- CDP core
  - request/response correlation
  - session-aware event dispatch
  - page-target routing

### Browser and page runtime

- `Browser`
  - launch local Chromium
  - connect to remote CDP in provider mode
  - own active persona and transport provider
- `Page`
  - navigation, evaluation, selectors, and extraction
  - deterministic and humanized interaction primitives
  - cookies, storage state, and debug snapshots

### Persona-driven stealth

`mindp` uses one persona as the source of truth for:

- locale and language
- timezone
- platform and browser identity
- screen and window metrics
- device-style values such as hardware concurrency and device memory
- user agent and client hints

That persona is then applied across five layers:

1. `surface`
2. `launch`
3. `network`
4. `transport`
5. `behavior`

The purpose is consistency. A stealth stack that reports one machine at launch, another in JavaScript, and a third in HTTP headers is usually worse than a smaller but coherent implementation.

## Stealth Model

### Surface

- generated JS patch bundle
- runtime/browser property normalization
- locale, screen, WebGL, permissions, and matchMedia adjustments
- audit support through `StealthReport`

### Launch

- local Chromium launch presets
- headful/headless operation
- profile shaping through user-data-dir selection
- provider seam for future external hardened runtimes

### Network

- user agent override
- accept-language coherence
- client hints metadata
- extra header policy and blocked URL policy

### Transport

- optional stdlib side-channel HTTP transport
- persona-bound defaults for `User-Agent` and `Accept-Language`
- separate from Chromium page traffic

### Behavior

- humanized mouse movement
- humanized click and text entry
- warmup flow and timing jitter
- deterministic fallback still available

## Media Flow

HLS support is intentionally simple:

- observe browser network events
- classify manifests and segments
- keep a manifest history per page
- start `ffmpeg` once a suitable manifest is known

This keeps media discovery inside the browser flow while leaving recording to a mature external tool.

## Design Constraints

The architecture deliberately favors:

- low dependency surface
- predictable ownership of behavior
- direct access to CDP when needed
- a thin public API that can be understood without learning a framework

It deliberately does not try to provide:

- full browser stealth equal to a patched browser build
- cross-browser abstraction at the API layer
- test-runner style orchestration
- hidden retries and hidden policy magic
