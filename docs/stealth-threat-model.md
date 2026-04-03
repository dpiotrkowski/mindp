# Stealth Threat Model

## Objective

The goal of `mindp` stealth is not “undetectable browsing.” The goal is to reduce obvious automation contradictions and make the runtime controllable, inspectable, and coherent for serious browser automation work.

## Native Scope

The current native scope includes:

- browser-surface consistency
- launch and profile consistency
- user agent and language coherence
- client-hints coherence
- state reuse
- timing jitter and humanized interaction primitives
- side-channel HTTP requests that inherit persona defaults

## Explicitly Out of Scope

`mindp` does not natively solve:

- browser-internal artifacts that require Chromium or Firefox patching
- TLS/JA3/ALPN impersonation for Chromium page traffic
- IP reputation and proxy trust
- CAPTCHA solving
- vendor-specific anti-bot bypass logic
- generalized anti-detect browser behavior

These are not accidental omissions. They are outside the design boundary of a small stdlib-first Go library.

## Threat Categories

### Surface detection

Examples:

- `navigator.webdriver`
- inconsistent screen/window metrics
- suspicious WebGL values
- locale or timezone mismatches

`mindp` addresses this category directly.

### Launch and profile detection

Examples:

- obviously automation-oriented launch flags
- contradictory headless characteristics
- unstable profile identity

`mindp` addresses this partially through presets and profile handling.

### Network and header detection

Examples:

- user agent and client hints disagreeing
- language headers not matching runtime values
- inconsistent request defaults across contexts

`mindp` addresses this category directly for browser-exposed values and side-channel HTTP defaults.

### Transport fingerprinting

Examples:

- TLS fingerprint mismatches
- ALPN and JA3 characteristics
- browser-vs-client HTTP stack differences

`mindp` does not solve this for browser page traffic.

### Behavioral detection

Examples:

- zero-latency actions
- unrealistic pointer movement
- no warmup or trust-building behavior

`mindp` addresses this partially through humanized action APIs and configurable timing.

## Recommended Operating Modes

### Moderate targets

Use:

- local Chromium
- balanced or stealth launch preset
- persona-aligned locale/timezone/language
- humanized actions where needed

### Hostile targets

Use:

- headful mode where possible
- persistent profiles where identity accumulation matters
- aligned proxy, locale, timezone, and language
- `StealthReport` and canary checks before broad deployment
- external hardened providers if browser-internal stealth becomes necessary

## Validation Strategy

Stealth should be treated as an empirical property:

- run fingerprint canaries
- capture snapshots and reports
- benchmark against target sites
- review failures as system behavior, not just as “missing one patch”
