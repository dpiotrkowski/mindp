// Package mindp provides small Chromium CDP automation in pure Go stdlib.
//
// The library is aimed at browser-driven data collection rather than test
// automation. Its core use cases are authenticated scraping, structured data
// extraction, browser-controlled interaction, HLS discovery, and managed media
// recording with minimal runtime and dependency surface.
//
// mindp is intentionally Chromium-first and narrow in scope. It provides:
//
//   - local browser launch and a remote CDP seam
//   - page automation primitives for navigation, evaluation, extraction, and
//     interaction
//   - persona-driven stealth across surface, launch, network, transport, and
//     behavior layers
//   - state persistence helpers for cookies and storage
//   - HLS detection and ffmpeg-backed recording
//
// It does not attempt to be a full generated CDP SDK, a test framework, or a
// patched anti-detect browser.
package mindp
