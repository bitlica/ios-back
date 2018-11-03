package reply

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testReplier struct{ JSONReplier }

func TestGetWhatWasSet(t *testing.T) {
	ctx := context.Background()
	replier := &testReplier{}

	ctx = NewContext(ctx, replier)
	got := FromContext(ctx)
	assert.Exactly(t, got, replier)
}

func TestAutoCreateReplier(t *testing.T) {
	ctx := context.Background()

	got := FromContext(ctx)
	assert.NotNil(t, got)
}

func TestJSONReplyOk(t *testing.T) {
	testcases := []struct {
		name       string
		input      interface{}
		expectCode int
		expectBody string
	}{
		{
			name:       "reply map",
			input:      map[string]interface{}{"key": "value"},
			expectBody: `{"key":"value"}`,
			expectCode: http.StatusOK,
		},
		{
			name:       "reply struct",
			input:      struct{ Key string }{"value"},
			expectBody: `{"Key":"value"}`,
			expectCode: http.StatusOK,
		},
		{
			name:       "reply string",
			input:      "{invalid json",
			expectBody: `"{invalid json"`,
			expectCode: http.StatusOK,
		},
		{
			name:       "reply integer",
			input:      1,
			expectBody: `1`,
			expectCode: http.StatusOK,
		},
		{
			name:       "reply reader",
			input:      bytes.NewBufferString("{invalid json"),
			expectBody: `{invalid json`,
			expectCode: http.StatusOK,
		},
		{
			name:       "unable marshal reply",
			input:      func() {},
			expectBody: `{"message":"internal error"}`,
			expectCode: http.StatusInternalServerError,
		},
	}

	ctx := context.Background()
	for _, tc := range testcases {
		t.Run(
			tc.name,
			func(t *testing.T) {
				w := httptest.NewRecorder()
				Ok(ctx, w, tc.input)
				assert.Equal(t, tc.expectCode, w.Code)
				assert.Equal(t, tc.expectBody, w.Body.String())
			},
		)
	}
}

func TestJSONReplyErr(t *testing.T) {
	ctx := context.Background()
	w := httptest.NewRecorder()

	Err(ctx, w, http.StatusTeapot, "some err")

	assert.Equal(t, http.StatusTeapot, w.Code)
	assert.Equal(t, `{"message":"some err"}`, w.Body.String())
}
