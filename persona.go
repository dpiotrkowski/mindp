package mindp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"runtime"
	"strings"
)

func resolvePersona(cfg Config, userDataDir string, browserVersion string) Persona {
	if cfg.Persona != nil {
		persona := *cfg.Persona
		return persona.withDefaults(cfg, userDataDir, browserVersion)
	}
	return Persona{}.withDefaults(cfg, userDataDir, browserVersion)
}

func (p Persona) withDefaults(cfg Config, userDataDir string, browserVersion string) Persona {
	p = applyCorePersonaDefaults(p, browserVersion)
	p = applyDisplayPersonaDefaults(p, cfg)
	p = applyDevicePersonaDefaults(p)
	if p.ID == "" {
		sum := sha256.Sum256(fmt.Appendf(nil, "%s|%s|%s|%s", userDataDir, p.Locale, p.Platform, p.BrowserVersion))
		p.ID = hex.EncodeToString(sum[:8])
	}
	if p.UserAgent == "" {
		p.UserAgent = defaultUserAgent(p)
	}
	return p
}

func applyCorePersonaDefaults(p Persona, browserVersion string) Persona {
	if p.BrowserName == "" {
		p.BrowserName = "Chrome"
	}
	if p.BrowserVersion == "" {
		p.BrowserVersion = browserVersion
	}
	if p.Locale == "" {
		p.Locale = "en-US"
	}
	if len(p.Languages) == 0 {
		p.Languages = []string{p.Locale, localeBase(p.Locale)}
	}
	if p.AcceptLanguage == "" {
		p.AcceptLanguage = strings.Join(uniqueStrings(p.Languages), ",")
	}
	if p.Timezone == "" {
		p.Timezone = "UTC"
	}
	if p.OS == "" {
		p.OS = defaultPersonaOS()
	}
	if p.Platform == "" {
		p.Platform = defaultPersonaPlatform(p.OS)
	}
	return p
}

func applyDisplayPersonaDefaults(p Persona, cfg Config) Persona {
	if p.WindowSize.Width == 0 || p.WindowSize.Height == 0 {
		p.WindowSize = cfg.WindowSize
	}
	if p.ScreenSize.Width == 0 || p.ScreenSize.Height == 0 {
		p.ScreenSize = p.WindowSize
	}
	if p.ColorScheme == "" {
		p.ColorScheme = "light"
	}
	if p.ReducedMotion == "" {
		p.ReducedMotion = "no-preference"
	}
	return p
}

func applyDevicePersonaDefaults(p Persona) Persona {
	if p.HardwareConcurrency == 0 {
		p.HardwareConcurrency = 8
	}
	if p.DeviceMemory == 0 {
		p.DeviceMemory = 8
	}
	if p.WebGLVendor == "" {
		p.WebGLVendor = "Intel Inc."
	}
	if p.WebGLRenderer == "" {
		p.WebGLRenderer = "Intel Iris OpenGL Engine"
	}
	return p
}

func defaultPersonaOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "windows":
		return "Windows"
	default:
		return "Linux"
	}
}

func defaultPersonaPlatform(os string) string {
	switch os {
	case "macOS":
		return "MacIntel"
	case "Windows":
		return "Win32"
	default:
		return "Linux x86_64"
	}
}

func defaultUserAgent(p Persona) string {
	version := p.BrowserVersion
	if version == "" {
		version = "120.0.0.0"
	}
	switch p.OS {
	case "macOS":
		return fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version)
	case "Windows":
		return fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version)
	default:
		return fmt.Sprintf("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", version)
	}
}

func localeBase(locale string) string {
	if idx := strings.IndexByte(locale, '-'); idx > 0 {
		return locale[:idx]
	}
	return locale
}

func uniqueStrings(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, item := range in {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
