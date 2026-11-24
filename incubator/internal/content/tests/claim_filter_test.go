package tests

import (
	contentapi "content-automation-engine/internal/content/api"
	"content-automation-engine/internal/content/repository"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestClaimFilterRejectsInvalidExcludedIDs(t *testing.T) {
	_, err := repository.ClaimFilter(contentapi.ClaimOptions{ExcludeIDs: []string{"not-an-object-id"}})

	require.Error(t, err)
}

func TestClaimFilterOnlyAddsTheConditionsAskedFor(t *testing.T) {
	now := time.Now()

	minimal, err := repository.ClaimFilter(contentapi.ClaimOptions{Now: now})
	require.NoError(t, err)
	full, err := repository.ClaimFilter(contentapi.ClaimOptions{
		Now:          now,
		MaxAttempts:  3,
		MinBodyChars: 10,
		MaxBodyChars: 100,
		ExcludeIDs:   []string{bson.NewObjectID().Hex()},
	})
	require.NoError(t, err)

	// Always: the claim check and the nsfw check. Optional: attempts, exclusions, min and max length.
	assert.Len(t, minimal[0].Value, 2)
	assert.Len(t, full[0].Value, 6)

	// The filter must be something the driver can actually send to the server.
	_, err = bson.Marshal(full)
	require.NoError(t, err)
}
