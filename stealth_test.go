package mindp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolvePersonaDefaults(t *testing.T) {
	cfg := Config{WindowSize: Size{Width: 1440, Height: 900}}.withDefaults()
	persona := resolvePersona(cfg, "/tmp/mindp-persona", "145.0.0.0")
	if persona.Locale != "en-US" {
		t.Fatalf("unexpected locale %q", persona.Locale)
	}
	if persona.Timezone != "UTC" {
		t.Fatalf("unexpected timezone %q", persona.Timezone)
	}
	if persona.UserAgent == "" {
		t.Fatal("expected user agent")
	}
	if persona.ID == "" {
		t.Fatal("expected persona id")
	}
}

func TestStdlibTransportBindsPersona(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != "UA/1.0" {
			t.Fatalf("unexpected user-agent %q", got)
		}
		if got := r.Header.Get("Accept-Language"); got != "pl-PL,pl" {
			t.Fatalf("unexpected accept-language %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := Config{
		Stealth: StealthPolicy{
			Transport: TransportProfile{BindPersona: true},
		},
	}.withDefaults()
	persona := Persona{
		UserAgent:      "UA/1.0",
		AcceptLanguage: "pl-PL,pl",
	}
	transport := newStdlibTransport(cfg, persona)
	resp, err := transport.Do(context.Background(), &TransportRequest{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != http.StatusNoContent {
		t.Fatalf("unexpected status %d", resp.Status)
	}
}
