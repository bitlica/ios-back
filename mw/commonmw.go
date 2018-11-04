package mw

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/Loofort/ios-back/log"
	"github.com/Loofort/ios-back/reply"
	"github.com/Loofort/ios-back/usage"
	"github.com/satori/go.uuid"
)

type startKeyType struct{}

var startKey = startKeyType{}

// CommonHandler set common midleware's capabilities:
// - add usage logging
// - add request id
// - add request time
func NewCommonHandler(handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// set start time
		ctx = context.WithValue(ctx, startKey, time.Now())

		// set request id
		reqid := uuid.NewV4().String()
		w.Header().Set("X-Request-ID", reqid)

		logger := log.FromContext(ctx)
		if lw, ok := logger.(interface {
			With(...interface{}) log.Logger
		}); ok {
			logger = lw.With("reqid", reqid)
		}
		ctx = log.NewContext(ctx, logger)

		// add usage
		ctx = usage.NewContext(ctx,
			"path", r.URL.Path,
			"clientIP", ExtractClientIP(r),
		)

		// inject new replier that reports usage
		replier := reply.FromContext(ctx)
		replier = usageReplier{replier}
		ctx = reply.NewContext(ctx, replier)

		handler.ServeHTTP(w, r.WithContext(ctx))
	}
}

// the client ip could be used for logging purpose and should not be used for any control decision since "X-Forwarded-For" could be abused by client
func ExtractClientIP(r *http.Request) string {
	// there could be multiple X-Forwarded-For and in general case it depends on your architecture what you can trust.
	// for the logging purpose we could get first one.
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	return clientIP
}

type usageReplier struct {
	reply.Replier
}

func (rpl usageReplier) Reply(ctx context.Context, w http.ResponseWriter, status int, result io.Reader) int {
	n := rpl.Reply(ctx, w, status, result)

	// log usage, add counted bytes
	start, _ := ctx.Value(startKey).(time.Time) // can't be error
	took := time.Since(start) / time.Millisecond

	usage := usage.FromContext(ctx)
	usage = append(usage,
		"status", status,
		"sent", n,
		"took", int(took),
	)
	log.FromContext(ctx).Info("usage", usage...)

	return n
}
