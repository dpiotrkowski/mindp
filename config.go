package mindp

import "time"

type Config struct {
	ExecutablePath string
	Headless       bool
	UserDataDir    string
	WindowSize     Size
	Proxy          string
	Args           []string
	Env            []string
	DisableStealth bool
	Persona        *Persona
	Stealth        StealthPolicy
	Provider       ProviderConfig
	Timeouts       Timeouts
}

type Size struct {
	Width  int
	Height int
}

type Timeouts struct {
	Startup time.Duration
	Action  time.Duration
}

func (c Config) withDefaults() Config {
	if c.WindowSize.Width == 0 {
		c.WindowSize.Width = 1440
	}
	if c.WindowSize.Height == 0 {
		c.WindowSize.Height = 900
	}
	if c.Timeouts.Startup <= 0 {
		c.Timeouts.Startup = 15 * time.Second
	}
	if c.Timeouts.Action <= 0 {
		c.Timeouts.Action = 20 * time.Second
	}
	if c.Provider.Kind == "" {
		c.Provider.Kind = ProviderKindLocalChromium
	}
	if c.Stealth.Level == "" {
		c.Stealth.Level = StealthLevelBalanced
	}
	if c.Stealth.Launch.Preset == "" {
		c.Stealth.Launch.Preset = LaunchPresetBalanced
	}
	if c.Stealth.Behavior.Mode == "" {
		c.Stealth.Behavior.Mode = TimingModeJittered
	}
	if c.Stealth.Behavior.MinDelay <= 0 {
		c.Stealth.Behavior.MinDelay = 35 * time.Millisecond
	}
	if c.Stealth.Behavior.MaxDelay <= 0 {
		c.Stealth.Behavior.MaxDelay = 120 * time.Millisecond
	}
	if c.Stealth.Behavior.MouseSteps <= 0 {
		c.Stealth.Behavior.MouseSteps = 8
	}
	if c.Stealth.Behavior.WarmupDelay <= 0 {
		c.Stealth.Behavior.WarmupDelay = 250 * time.Millisecond
	}
	if !c.Stealth.Transport.BindPersona {
		c.Stealth.Transport.BindPersona = true
	}
	return c
}
