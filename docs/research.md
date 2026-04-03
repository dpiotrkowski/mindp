# Research

The initial library survey that informed `mindp` lives below.

---

# Browser Automation Research (Go / Rust)

Date: 2026-04-03

## Scope

Compared current Go and Rust browser automation libraries for:

- data-heavy automation and scraping
- browser-driven media/video workflows
- preference for CDP-first design
- smaller dependency trees over large frameworks
- active development in 2026
- production stability

## Local repos cloned

- `chromedp`
- `rod`
- `playwright-go`
- `chromiumoxide`
- `rust-headless-chrome`
- `fantoccini`
- `thirtyfour`
- `mafredri-cdp`
- `tebeka-selenium`

## Main takeaways

- There is no clean, small, CDP-first library that gives first-class Firefox support in Go or Rust.
- For Chromium, the best fits were `chromedp` and `rod`.
- For Firefox, the realistic path is WebDriver or BiDi rather than CDP.
- For richer capture flows, `rod` exposes more screencast/tracing primitives directly.
