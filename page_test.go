package mindp

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestBuildExpression(t *testing.T) {
	got := buildExpression("(a, b) => a + b", []any{1, "x"})
	if !strings.Contains(got, `((a, b) => a + b)(1,"x")`) {
		t.Fatalf("unexpected expression %q", got)
	}
}

func TestDecodeString(t *testing.T) {
	raw := json.RawMessage(`"hello"`)
	got, err := decodeString(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestLaunchAndNavigateIntegration(t *testing.T) {
	if _, err := exec.LookPath("chromium"); err != nil {
		t.Skip("chromium not installed")
	}
	ctx := context.Background()
	browser, err := Launch(ctx, Config{Headless: true})
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.NewPage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := page.Navigate(ctx, `data:text/html,<html><body><div id="msg">hello</div></body></html>`); err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(ctx); err != nil {
		t.Fatal(err)
	}
	text, err := page.Text(ctx, "#msg")
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello" {
		t.Fatalf("got %q", text)
	}
}
