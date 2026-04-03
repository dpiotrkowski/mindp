package mindp

import (
	"context"
	"net/http"
	"time"
)

type StealthLevel string

const (
	StealthLevelMinimal    StealthLevel = "minimal"
	StealthLevelBalanced   StealthLevel = "balanced"
	StealthLevelAggressive StealthLevel = "aggressive"
)

type LaunchPreset string

const (
	LaunchPresetDebug    LaunchPreset = "debug"
	LaunchPresetBalanced LaunchPreset = "balanced"
	LaunchPresetStealth  LaunchPreset = "stealth"
	LaunchPresetHostile  LaunchPreset = "hostile"
)

type TimingMode string

const (
	TimingModeDeterministic TimingMode = "deterministic"
	TimingModeJittered      TimingMode = "jittered"
	TimingModeHumanized     TimingMode = "humanized"
)

type ProviderKind string

const (
	ProviderKindLocalChromium ProviderKind = "local-chromium"
	ProviderKindRemoteCDP     ProviderKind = "remote-cdp"
)

type Persona struct {
	ID                  string   `json:"id,omitempty"`
	BrowserName         string   `json:"browserName,omitempty"`
	BrowserVersion      string   `json:"browserVersion,omitempty"`
	OS                  string   `json:"os,omitempty"`
	Platform            string   `json:"platform,omitempty"`
	Locale              string   `json:"locale,omitempty"`
	Languages           []string `json:"languages,omitempty"`
	Timezone            string   `json:"timezone,omitempty"`
	UserAgent           string   `json:"userAgent,omitempty"`
	AcceptLanguage      string   `json:"acceptLanguage,omitempty"`
	WindowSize          Size     `json:"windowSize"`
	ScreenSize          Size     `json:"screenSize"`
	ColorScheme         string   `json:"colorScheme,omitempty"`
	ReducedMotion       string   `json:"reducedMotion,omitempty"`
	HardwareConcurrency int      `json:"hardwareConcurrency,omitempty"`
	DeviceMemory        int      `json:"deviceMemory,omitempty"`
	WebGLVendor         string   `json:"webglVendor,omitempty"`
	WebGLRenderer       string   `json:"webglRenderer,omitempty"`
}

type StealthPolicy struct {
	Level     StealthLevel     `json:"level,omitempty"`
	Launch    LaunchProfile    `json:"launch"`
	Network   NetworkPolicy    `json:"network"`
	Behavior  BehaviorProfile  `json:"behavior"`
	Transport TransportProfile `json:"transport"`
}

type LaunchProfile struct {
	Preset            LaunchPreset `json:"preset,omitempty"`
	PersistentProfile bool         `json:"persistentProfile,omitempty"`
}

type NetworkPolicy struct {
	Headers        map[string]string `json:"headers,omitempty"`
	BlockedURLs    []string          `json:"blockedUrls,omitempty"`
	UserAgent      string            `json:"userAgent,omitempty"`
	AcceptLanguage string            `json:"acceptLanguage,omitempty"`
	ClientHints    ClientHintsPolicy `json:"clientHints"`
}

type ClientHintsPolicy struct {
	Platform        string `json:"platform,omitempty"`
	PlatformVersion string `json:"platformVersion,omitempty"`
	Architecture    string `json:"architecture,omitempty"`
	Model           string `json:"model,omitempty"`
	Mobile          bool   `json:"mobile,omitempty"`
}

type BehaviorProfile struct {
	Mode        TimingMode    `json:"mode,omitempty"`
	MinDelay    time.Duration `json:"minDelay,omitempty"`
	MaxDelay    time.Duration `json:"maxDelay,omitempty"`
	MouseSteps  int           `json:"mouseSteps,omitempty"`
	WarmupDelay time.Duration `json:"warmupDelay,omitempty"`
}

type TransportProfile struct {
	BindPersona    bool              `json:"bindPersona,omitempty"`
	DefaultHeaders map[string]string `json:"defaultHeaders,omitempty"`
	Proxy          string            `json:"proxy,omitempty"`
}

type ProviderConfig struct {
	Kind     ProviderKind `json:"kind,omitempty"`
	DebugURL string       `json:"debugURL,omitempty"`
}

type StealthReport struct {
	Persona Persona        `json:"persona"`
	Values  map[string]any `json:"values"`
	Issues  []string       `json:"issues,omitempty"`
}

type TransportRequest struct {
	Method  string
	URL     string
	Headers http.Header
	Body    []byte
}

type TransportResponse struct {
	Status  int
	Headers http.Header
	Body    []byte
}

type TransportProvider interface {
	Do(ctx context.Context, req *TransportRequest) (*TransportResponse, error)
}
