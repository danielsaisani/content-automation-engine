package tests

import (
	"content-automation-engine/internal/events"
	"content-automation-engine/internal/scheduler"
	"content-automation-engine/internal/testutil"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runScheduler(t *testing.T, cfg *scheduler.Config) chan events.TopicTriggered {
	t.Helper()
	deps, _ := testutil.NewDependencies()
	topics := make(chan events.TopicTriggered, 10)
	service, err := scheduler.NewRealSchedulerService(deps, topics, cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	return topics
}

func TestSchedulerFiresOnEveryInterval(t *testing.T) {
	topics := runScheduler(t, &scheduler.Config{Interval: 20 * time.Millisecond})

	for i := range 3 {
		select {
		case trigger := <-topics:
			assert.Equal(t, events.TopicPayload("misc"), trigger.Topic)
			assert.NotEmpty(t, trigger.ID)
			assert.False(t, trigger.TriggeredAt.IsZero())
		case <-time.After(time.Second):
			t.Fatalf("trigger %d never arrived", i+1)
		}
	}
}

func TestSchedulerWaitsAFullIntervalByDefault(t *testing.T) {
	topics := runScheduler(t, &scheduler.Config{Interval: time.Hour})

	select {
	case <-topics:
		t.Fatal("the scheduler fired before its first interval had passed")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSchedulerCanFireOnStart(t *testing.T) {
	topics := runScheduler(t, &scheduler.Config{Interval: time.Hour, RunOnStart: true})

	select {
	case <-topics:
	case <-time.After(time.Second):
		t.Fatal("the scheduler did not fire on start")
	}
}

func TestSchedulerStopsEvenWhenNobodyIsListening(t *testing.T) {
	deps, _ := testutil.NewDependencies()
	unbuffered := make(chan events.TopicTriggered)
	service, err := scheduler.NewRealSchedulerService(deps, unbuffered, &scheduler.Config{Interval: time.Hour, RunOnStart: true})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	time.Sleep(20 * time.Millisecond) // the scheduler is now blocked trying to send
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("the scheduler did not stop")
	}
}

func TestSchedulerRejectsANonPositiveInterval(t *testing.T) {
	deps, _ := testutil.NewDependencies()

	_, err := scheduler.NewRealSchedulerService(deps, make(chan events.TopicTriggered), &scheduler.Config{})

	require.Error(t, err)
}

func TestLoadSchedulerConfigReadsThePrefixedVariable(t *testing.T) {
	t.Setenv("SCHEDULER_INTERVAL", "90m")

	cfg, err := scheduler.LoadConfig()

	require.NoError(t, err)
	assert.Equal(t, 90*time.Minute, cfg.Interval)
	assert.False(t, cfg.RunOnStart)
}

func TestLoadSchedulerConfigRequiresAnInterval(t *testing.T) {
	_, err := scheduler.LoadConfig()

	require.Error(t, err)
}

func TestLoadSchedulerConfigRejectsABadInterval(t *testing.T) {
	t.Setenv("SCHEDULER_INTERVAL", "soon")

	_, err := scheduler.LoadConfig()

	require.Error(t, err)
}
