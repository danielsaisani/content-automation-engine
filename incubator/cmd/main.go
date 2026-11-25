package main

import (
	"content-automation-engine/internal/app"
	"context"
	"log"
	"os"
)

func main() {
	contentAutomationEngine := app.ContentAutomationEngineApplication{}
	if err := contentAutomationEngine.Start(context.TODO()); err != nil {
		log.Printf("content automation engine failed: %v", err)
		os.Exit(1)
	}
}
