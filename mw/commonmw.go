package mw

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Loofort/ios-back/log"
	"github.com/Loofort/ios-back/reply"
	"github.com/Loofort/ios-back/usage"
	"github.com/satori/go.uuid"
)

// CommonHandler set common midleware's capabilities:
// - add usage logging
// - add request id
// - add request time
func NewCommonHandler(handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// inject new replier that reports usage
		replier := reply.FromContext(ctx)
		replier = usageReplier{replier, time.Now()}
		ctx = reply.NewContext(ctx, replier)

		// set request id
		reqid := uuid.NewV4().String()
		w.Header().Set("X-Request-ID", reqid)
		ctx = log.With(ctx, "reqid", reqid)

		// add usage
		ctx = usage.NewContext(ctx,
			"path", r.URL.Path,
			"clientIP", ExtractClientIP(r),
		)

		handler.ServeHTTP(w, r.WithContext(ctx))
	}
}

// the client ip could be used for logging purpose and should not be used for any control decision since "X-Forwarded-For" could be abused by client
func ExtractClientIP(r *http.Request) string {
	// there could be multiple X-Forwarded-For and in general case it depends on your architecture what you can trust.
	// for the logging purpose we could get first one.
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = strings.Split(r.RemoteAddr, ":")[0]
	}
	return clientIP
}

type usageReplier struct {
	reply.Replier
	start time.Time
}

func (rpl usageReplier) Reply(ctx context.Context, w http.ResponseWriter, status int, result io.Reader) int {
	n := rpl.Replier.Reply(ctx, w, status, result)

	// log usage, add counted bytes
	took := time.Since(rpl.start) / time.Millisecond

	usage := usage.FromContext(ctx)
	usage = append(usage,
		"status", status,
		"sent", n,
		"took", int(took),
	)
	log.Info(ctx, "usage", usage...)

	return n
}
