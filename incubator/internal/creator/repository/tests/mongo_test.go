package tests

import (
	creator "content-automation-engine/internal/creator/repository"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionURLBuilderProd(t *testing.T) {
	urlBuilder := creator.ConnectionURLBuilder{}

	connectionURL := urlBuilder.
		Method("prod").
		Credentials("user", "password").
		Host("prod.skbzy7n.mongodb.net").
		App("prod").
		Build()

	require.NotEmpty(t, connectionURL)
	assert.Equal(t, "mongodb+srv://user:password@prod.skbzy7n.mongodb.net/?appName=prod", connectionURL)
}

func TestConnectionURLBuilderDev(t *testing.T) {
	urlBuilder := creator.ConnectionURLBuilder{}

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
