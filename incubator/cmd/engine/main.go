package main

import (
	"content-automation-engine/cmd/application"
	"content-automation-engine/cmd/config"
	"content-automation-engine/internal/creator"
	"content-automation-engine/internal/creator/scraper"
	"content-automation-engine/internal/events"
	"content-automation-engine/internal/observability/notifier"
	"content-automation-engine/internal/scheduler"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := config.NewConfig()
	if err := cfg.Load(context.Background()); err != nil {
		log.Fatal(err)
	}
	subreddits := []string{
		"worldnews", "news", "politics", "europe", "ukpolitics",
		"conservative", "liberal", "geopolitics", "upliftingnews", "nottheonion",
		"nfl", "nba", "soccer", "baseball", "hockey",
		"formula1", "tennis", "MMA", "cricket", "fantasyfootball",
		"gaming", "pcgaming", "PS5", "XboxSeriesX", "leagueoflegends",
		"Minecraft", "Fortnite", "GlobalOffensive", "Competitiveoverwatch", "DotA2",
		"technology", "programming", "Python", "javascript", "webdev",
		"artificial", "MachineLearning", "ChatGPT", "cybersecurity", "linux",
		"movies", "television", "anime", "marvelstudios", "StarWars",
		"NetflixBestOf", "popheads", "hiphopheads", "music", "books",
		"AskReddit", "AmItheAsshole", "relationship_advice", "dating_advice", "tifu",
		"confession", "unpopularopinion", "changemyview", "LifeProTips", "self",
		"wallstreetbets", "investing", "personalfinance", "CryptoCurrency", "Bitcoin",
		"ethereum", "stocks", "financialindependence", "frugal", "eupersonalfinance",
		"memes", "dankmemes", "funny", "me_irl", "facepalm",
		"therewasanattempt", "Unexpected", "instant_regret", "nextfuckinglevel", "oddlysatisfying",
		"science", "space", "history", "todayilearned", "explainlikeimfive",
		"askscience", "medicine", "psychology", "philosophy", "economics",
		"pics", "gifs", "videos", "WTF", "interestingasfuck",
		"mildlyinteresting", "photoshopbattles", "DIY", "food", "aww", "AITAH",
	}

	redditScraper := scraper.NewRedditScraper(subreddits)

	redditScraper.Run(context.Background())

	serviceDependencies := application.NewServiceDependencies()

	serviceDependencies.Logger.Info("Starting engine..")

	serviceDependencies.Logger.Info("Creating story repository")

	topicCh := make(chan events.TopicTriggered, 10)
	storyCh := make(chan events.StoryScraped, 10)

	// TOOD: run all services from main thread one by one
	// var services []api.Service

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler := scheduler.NewRealScheduler(serviceDependencies, topicCh)
	go scheduler.Run(ctx)

	notifier := notifier.NewNotifierService(serviceDependencies, topicCh)
	go notifier.Run(ctx)

	creator := creator.NewCreatorService(serviceDependencies, topicCh, storyCh, cfg.StoryRepository, redditScraper)
	go creator.Run(ctx)

	serviceDependencies.Logger.Info("Engine started..")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	serviceDependencies.Logger.Info("Engine stopping..")
}
