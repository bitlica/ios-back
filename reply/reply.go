package reply

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/Loofort/ios-back/log"
	"github.com/Loofort/ios-back/usage"
)

// New produces replier object and can be redefined.
var New = func() Replier {
	return JSONReplier{}
}

type Replier interface {
	// Reply is the function that send response to the user.
	Reply(ctx context.Context, w http.ResponseWriter, status int, response interface{})
	// Ok produces output with http.StatusOK http code.  It is shorthand for Reply(ctx, w, http.StatusOK, response).
	Ok(ctx context.Context, w http.ResponseWriter, response interface{})
	// Err formats error response based on passed message
	Err(ctx context.Context, w http.ResponseWriter, status int, errmsg string)
}

type JSONReplier struct{}

func (JSONReplier) reply(ctx context.Context, w http.ResponseWriter, status int, response io.Reader) {
	logger := log.FromContext(ctx)

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)

	n, err := io.Copy(w, response)
	if err != nil {
		logger.Error("unable to reply", "err", err, "type", "reply")
	}

	// log usage
	usage := usage.FromContext(ctx)
	usage = append(usage,
		"http_status", status,
		"sent", n,
	)
	logger.Info("usage", usage...)
}

func (rpl JSONReplier) Reply(ctx context.Context, w http.ResponseWriter, status int, response interface{}) {
	// check reader
	if reader, ok := response.(io.Reader); ok {
		rpl.reply(ctx, w, status, reader)
		return
	}

	// marshal body
	data, err := json.Marshal(response)
	if err != nil {
		log.FromContext(ctx).Error("unable marshal response", "err", err)
		rpl.Err(ctx, w, http.StatusInternalServerError, "internal error")
		return
	}

	//create reader
	reader := bytes.NewBuffer(data)
	rpl.reply(ctx, w, status, reader)
}

func (rpl JSONReplier) Ok(ctx context.Context, w http.ResponseWriter, response interface{}) {
	rpl.Reply(ctx, w, http.StatusOK, response)
}

func (rpl JSONReplier) Err(ctx context.Context, w http.ResponseWriter, status int, errmsg string) {
	apierr := struct {
		Message string `json:"message"`
	}{errmsg}

	data, _ := json.Marshal(apierr)
	reader := bytes.NewBuffer(data)
	rpl.reply(ctx, w, status, reader)
}

/*************** context *************/
type replyKeyType struct{}

var replyKey = replyKeyType{}

func NewContext(ctx context.Context, replier Replier) context.Context {
	return context.WithValue(ctx, replyKey, replier)
}

func FromContext(ctx context.Context) Replier {
	replier, _ := ctx.Value(replyKey).(Replier)
	if replier == nil {
		replier = New()
	}

	return replier
}
