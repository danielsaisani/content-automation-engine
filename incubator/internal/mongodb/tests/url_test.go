package tests

import (
	"content-automation-engine/internal/mongodb"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionURLBuilderProd(t *testing.T) {
	urlBuilder := mongodb.ConnectionURLBuilder{}

	connectionURL := urlBuilder.
		Method("prod").
		Credentials("user", "password").
		Host("prod.example.mongodb.net").
		App("prod").
		Build()

	require.NotEmpty(t, connectionURL)
	assert.Equal(t, "mongodb+srv://user:password@prod.example.mongodb.net/?appName=prod", connectionURL)
}

func TestConnectionURLBuilderDev(t *testing.T) {
	urlBuilder := mongodb.ConnectionURLBuilder{}

	connectionURL := urlBuilder.
		Method("dev").
		Credentials("user", "password").
		Host("localhost:27017").
		App("dev").
		Build()

	require.NotEmpty(t, connectionURL)
	assert.Equal(t, "mongodb://user:password@localhost:27017/?appName=dev", connectionURL)
	assert.NotContains(t, connectionURL, "+srv")
}

func TestConnectionURLEscapesCredentials(t *testing.T) {
	cfg := &mongodb.Config{Username: "us:er", Password: "p@ss/word", Host: "localhost:27017", AppName: "dev"}

	assert.Equal(t, "mongodb://us%3Aer:p%40ss%2Fword@localhost:27017/?appName=dev", cfg.ConnectionURL("dev"))
}

func TestLoadConfig(t *testing.T) {
	t.Setenv("MONGO_USERNAME", "user")
	t.Setenv("MONGO_PASSWORD", "password")
	t.Setenv("MONGO_HOST", "localhost:27017")
	t.Setenv("MONGO_APP_NAME", "engine")

	cfg, err := mongodb.LoadConfig()

	require.NoError(t, err)
	assert.Equal(t, "engine", cfg.AppName)
	assert.Equal(t, "prod", cfg.Database, "database defaults to prod so existing deployments keep their data")
}

func TestLoadConfigMissingHost(t *testing.T) {
	t.Setenv("MONGO_USERNAME", "user")
	t.Setenv("MONGO_PASSWORD", "password")

	_, err := mongodb.LoadConfig()

	require.Error(t, err)
}
