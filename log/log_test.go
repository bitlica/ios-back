package log

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testLogget struct{ dummyLogger }

func TestGetWhatWasSet(t *testing.T) {
	ctx := context.Background()
	logger := &testLogget{}

	ctx = NewContext(ctx, logger)
	got := FromContext(ctx)
	assert.Exactly(t, got, logger)
}

func TestAutoCreateLogger(t *testing.T) {
	ctx := context.Background()

	got := FromContext(ctx)
	assert.NotNil(t, got)
}
