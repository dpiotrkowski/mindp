package main

import (
	"context"
	"fmt"
	"log"

	"mindp"
)

func main() {
	ctx := context.Background()
	browser, err := mindp.Launch(ctx, mindp.Config{Headless: false})
	if err != nil {
		log.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.NewPage(ctx)
	if err != nil {
		log.Fatal(err)
	}
	if err := page.Navigate(ctx, "https://example.com"); err != nil {
		log.Fatal(err)
	}
	if err := page.WaitLoad(ctx); err != nil {
		log.Fatal(err)
	}
	html, err := page.HTML(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("html bytes:", len(html))
}
