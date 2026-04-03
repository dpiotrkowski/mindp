package main

import (
	"context"
	"log"
	"time"

	"mindp"
)

func main() {
	ctx := context.Background()
	browser, err := mindp.Launch(ctx, mindp.Config{Headless: true})
	if err != nil {
		log.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.NewPage(ctx)
	if err != nil {
		log.Fatal(err)
	}
	stop := page.OnHLS(func(event mindp.HLSEvent) {
		log.Printf("hls %s %s", event.Kind, event.URL)
	})
	defer stop()

	if err := page.Navigate(ctx, "https://example.com"); err != nil {
		log.Fatal(err)
	}
	if err := page.WaitLoad(ctx); err != nil {
		log.Fatal(err)
	}

	_, _ = page.RecordHLS(context.Background(), mindp.HLSConfig{
		OutputPath: "capture.ts",
	})
	time.Sleep(2 * time.Second)
}
