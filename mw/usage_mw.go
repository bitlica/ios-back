package mw

import (
	"context"
	"io"
	"net/http"

	"github.com/Loofort/ios-back/log"
	"github.com/Loofort/ios-back/reply"
	"github.com/Loofort/ios-back/usage"
)

func NewUsageHandler(handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// inject new replier that reports usage
		ctx := r.Context()
		replier := reply.FromContext(ctx)
		replier = usageReplier{replier}
		ctx = reply.NewContext(ctx, replier)

		handler.ServeHTTP(w, r.WithContext(ctx))
	}
}

type usageReplier struct {
	reply.Replier
}

func (rpl usageReplier) Reply(ctx context.Context, w http.ResponseWriter, status int, result io.Reader) {
	// set wrapper for result reader to count body bytes
	counter := &counter{Reader: result}
	if wt, ok := result.(io.WriterTo); ok {
		counter.WriterTo = wt
		result = counter
	} else {
		result = readerOnly{counter}
	}

	rpl.Reply(ctx, w, status, result)

	// log usage, add counted bytes
	usage := usage.FromContext(ctx)
	usage = append(usage,
		"http_status", status,
		"sent", counter.count,
	)
	log.FromContext(ctx).Info("usage", usage...)
}

////////////////////
type counter struct {
	io.Reader
	io.WriterTo
	count int
}

func (c *counter) Read(p []byte) (n int, err error) {
	n, err = c.Reader.Read(p)
	c.count += n
	return n, err
}
func (c *counter) WriteTo(w io.Writer) (n int64, err error) {
	n, err = c.WriterTo.WriteTo(w)
	c.count += int(n)
	return n, err
}

type readerOnly struct {
	*counter
}
