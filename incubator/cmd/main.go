package main

import (
	"content-automation-engine/internal/app"
	"context"
)

func main() {
	contentAutomationEngine := app.ContentAutomationEngineApplication{}
	contentAutomationEngine.Start(context.TODO())
}
