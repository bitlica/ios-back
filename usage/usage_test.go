package usage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsage(t *testing.T) {
	testcases := []struct {
		name    string
		ctx     context.Context
		keyvals []interface{}
		expect  []interface{}
	}{
		{
			name:    "empty context",
			ctx:     context.Background(),
			keyvals: []interface{}{"foo", "bar"},
			expect:  []interface{}{"foo", "bar"},
		},
		{
			name:    "context with usage",
			ctx:     NewContext(context.Background(), "key", "val"),
			keyvals: []interface{}{"foo", "bar"},
			expect:  []interface{}{"key", "val", "foo", "bar"},
		},
		{
			name:    "odd number of keyvalues",
			ctx:     context.Background(),
			keyvals: []interface{}{"foo", "bar", "onlyKey"},
			expect:  []interface{}{"foo", "bar", "onlyKey", nil},
		},
		{
			name:    "set empty usage",
			ctx:     context.Background(),
			keyvals: []interface{}{},
			expect:  nil,
		},
		{
			name:    "set nil usage",
			ctx:     context.Background(),
			keyvals: nil,
			expect:  nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewContext(tc.ctx, tc.keyvals...)
			u := FromContext(ctx)
			assert.Equal(t, tc.expect, u)
		})
	}
}
