# Stealth Research

## Positioning

`mindp` is designed as a small Go-native automation runtime with a coherent stealth model. It is not intended to replace patched browsers or large anti-detect platforms.

That distinction matters:

- patched browser projects solve browser-internal artifacts that a normal CDP client often cannot hide cleanly
- transport impersonation projects solve HTTP and TLS identity problems outside the browser
- `mindp` solves runtime coordination and operator control inside a small automation library

## Why `mindp` Uses a Five-Layer Model

The five layers are:

1. `surface`
2. `launch`
3. `network`
4. `transport`
5. `behavior`

This model exists because stealth failures rarely come from one leak alone. They usually come from contradictions:

- browser flags imply one environment
- JavaScript reports another
- headers suggest a third
- user behavior looks synthetic

The persona model gives these layers one shared source of truth.

## What `mindp` Tries to Solve Well

- browser-surface consistency
- launch/profile shaping for Chromium automation
- language, timezone, UA, and client-hint coherence
- state reuse and humanized interaction timing
- browser-discovered media workflows
- side-channel stdlib HTTP requests that remain aligned with the active persona

## What `mindp` Does Not Claim to Solve

- Chromium internal artifacts that require browser patching
- browser TLS / JA3 / ALPN impersonation
- network reputation and proxy quality
- CAPTCHA solving
- vendor-specific anti-bot bypasses

Those are better handled by external hardened providers or specialized transport layers.

## Relationship to External Systems

### Patchright-style environments

Relevant because they address automation-runtime visibility and browser-level signals that a normal CDP client often cannot fully mask.

### Camoufox-style environments

Relevant because they represent the “patched browser” end of the design spectrum, where the runtime itself is modified to present a more believable profile.

### Lightpanda-style backends

Relevant as an experimental backend category. They may become useful where a CDP-compatible alternative runtime is desirable, but they are not the current baseline for `mindp`.

### `tls-client` / `impit`-style transports

Relevant for non-browser HTTP identity. They address a different layer than browser-surface stealth and should be treated as complementary, not interchangeable.

## Practical Conclusion

`mindp` should remain:

- small
- explicit
- persona-driven
- Chromium-first

The strongest long-term shape is:

- native `mindp` for small, owned Chromium automation
- optional remote-provider seam for hardened external browsers
- optional external transport providers for side-channel HTTP impersonation

That keeps the core maintainable while still leaving a path to stronger hostile-target operation.
