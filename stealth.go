package mindp

import (
	"encoding/json"
	"fmt"
	"strings"
)

func stealthSource(cfg Config) string {
	persona := resolvePersona(cfg, cfg.UserDataDir, "")
	payload := map[string]any{
		"level":               cfg.Stealth.Level,
		"locale":              persona.Locale,
		"languages":           uniqueStrings(persona.Languages),
		"platform":            persona.Platform,
		"userAgent":           effectiveUserAgent(cfg, persona),
		"hardwareConcurrency": persona.HardwareConcurrency,
		"deviceMemory":        persona.DeviceMemory,
		"colorScheme":         persona.ColorScheme,
		"reducedMotion":       persona.ReducedMotion,
		"screenWidth":         persona.ScreenSize.Width,
		"screenHeight":        persona.ScreenSize.Height,
		"windowWidth":         persona.WindowSize.Width,
		"windowHeight":        persona.WindowSize.Height,
		"webglVendor":         persona.WebGLVendor,
		"webglRenderer":       persona.WebGLRenderer,
		"timezone":            persona.Timezone,
	}
	data, _ := json.Marshal(payload)
	return fmt.Sprintf(`(() => {
const cfg = %s;
const define = (obj, key, getter) => {
  try {
    Object.defineProperty(obj, key, { get: getter, configurable: true });
  } catch (_) {}
};
const patchValue = (obj, key, value) => define(obj, key, () => value);
patchValue(Navigator.prototype, 'webdriver', undefined);
patchValue(Navigator.prototype, 'language', cfg.locale);
patchValue(Navigator.prototype, 'languages', cfg.languages);
patchValue(Navigator.prototype, 'platform', cfg.platform);
patchValue(Navigator.prototype, 'userAgent', cfg.userAgent);
patchValue(Navigator.prototype, 'hardwareConcurrency', cfg.hardwareConcurrency);
patchValue(Navigator.prototype, 'deviceMemory', cfg.deviceMemory);
patchValue(Screen.prototype, 'width', cfg.screenWidth);
patchValue(Screen.prototype, 'height', cfg.screenHeight);
patchValue(Screen.prototype, 'availWidth', cfg.windowWidth);
patchValue(Screen.prototype, 'availHeight', cfg.windowHeight);
patchValue(window, 'outerWidth', cfg.windowWidth);
patchValue(window, 'outerHeight', cfg.windowHeight);
patchValue(window, 'innerWidth', cfg.windowWidth);
patchValue(window, 'innerHeight', cfg.windowHeight);
if (!window.chrome) window.chrome = {};
if (!window.chrome.runtime) window.chrome.runtime = {};
if (navigator.permissions && navigator.permissions.query) {
  const originalQuery = navigator.permissions.query.bind(navigator.permissions);
  navigator.permissions.query = (parameters) => {
    if (parameters && parameters.name === 'notifications') {
      return Promise.resolve({ state: Notification.permission, onchange: null });
    }
    return originalQuery(parameters);
  };
}
if (!navigator.plugins || navigator.plugins.length === 0) {
  patchValue(Navigator.prototype, 'plugins', [1, 2, 3, 4, 5]);
}
const originalResolved = Intl.DateTimeFormat.prototype.resolvedOptions;
Intl.DateTimeFormat.prototype.resolvedOptions = function(...args) {
  const out = originalResolved.apply(this, args);
  out.timeZone = cfg.timezone;
  return out;
};
const originalGetParameter = WebGLRenderingContext.prototype.getParameter;
WebGLRenderingContext.prototype.getParameter = function(parameter) {
  if (parameter === 37445) return cfg.webglVendor;
  if (parameter === 37446) return cfg.webglRenderer;
  return originalGetParameter.call(this, parameter);
};
if (cfg.level === 'aggressive') {
  const originalOpen = XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open = function(...args) {
    return originalOpen.apply(this, args);
  };
}
const originalMatchMedia = window.matchMedia.bind(window);
window.matchMedia = (query) => {
  if (query === '(prefers-color-scheme: dark)') {
    return { matches: cfg.colorScheme === 'dark', media: query, onchange: null, addListener() {}, removeListener() {}, addEventListener() {}, removeEventListener() {}, dispatchEvent() { return false; } };
  }
  if (query === '(prefers-reduced-motion: reduce)') {
    return { matches: cfg.reducedMotion === 'reduce', media: query, onchange: null, addListener() {}, removeListener() {}, addEventListener() {}, removeEventListener() {}, dispatchEvent() { return false; } };
  }
  return originalMatchMedia(query);
};
})();`, string(data))
}

func effectiveUserAgent(cfg Config, persona Persona) string {
	if cfg.Stealth.Network.UserAgent != "" {
		return cfg.Stealth.Network.UserAgent
	}
	return persona.UserAgent
}

func effectiveAcceptLanguage(cfg Config, persona Persona) string {
	if cfg.Stealth.Network.AcceptLanguage != "" {
		return cfg.Stealth.Network.AcceptLanguage
	}
	if persona.AcceptLanguage != "" {
		return persona.AcceptLanguage
	}
	return strings.Join(uniqueStrings(persona.Languages), ",")
}

func userAgentMetadata(cfg Config, persona Persona) map[string]any {
	version := persona.BrowserVersion
	if version == "" {
		version = "120.0.0.0"
	}
	major := version
	if idx := strings.IndexByte(version, '.'); idx > 0 {
		major = version[:idx]
	}
	ch := cfg.Stealth.Network.ClientHints
	platform := ch.Platform
	if platform == "" {
		platform = persona.OS
	}
	arch := ch.Architecture
	if arch == "" {
		arch = "x86"
	}
	platformVersion := ch.PlatformVersion
	if platformVersion == "" {
		platformVersion = "0.0.0"
	}
	return map[string]any{
		"brands": []map[string]string{
			{"brand": "Chromium", "version": major},
			{"brand": "Google Chrome", "version": major},
			{"brand": "Not.A/Brand", "version": "99"},
		},
		"fullVersion":     version,
		"platform":        platform,
		"platformVersion": platformVersion,
		"architecture":    arch,
		"model":           ch.Model,
		"mobile":          ch.Mobile,
	}
}
