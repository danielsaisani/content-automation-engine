package tiktok

import (
	"context"
	"fmt"
)

// Post implements the Poster interface.
// It expects input to be of type *TiktokPostInput.
func (c *TiktokClient) Post(ctx context.Context, input interface{}) error {
	tiktokInput, ok := input.(*TiktokPostInput)
	if !ok {
		return fmt.Errorf("invalid input type: expected *TiktokPostInput, got %T", input)
	}

	return c.UploadVideo(ctx, tiktokInput)
}
