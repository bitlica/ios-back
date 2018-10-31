package usage

import "context"

type usageKeyType struct{}

var usageKey = usageKeyType{}

func NewContext(ctx context.Context, keyValues ...interface{}) context.Context {
	if len(keyValues)%2 != 0 {
		keyValues = append(keyValues, nil)
	}

	u := append(FromContext(ctx), keyValues...)
	return context.WithValue(ctx, usageKey, u)
}

func FromContext(ctx context.Context) []interface{} {
	u, _ := ctx.Value(usageKey).([]interface{})
	return u
}
