package reply

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/Loofort/ios-back/log"
)

// New produces replier object and can be redefined.
var New = func() Replier {
	return JSONReplier{}
}

type Replier interface {
	// Reply is the function that send response to the user.
	Reply(ctx context.Context, w http.ResponseWriter, response interface{}, status int)
	// Ok produces output with http.StatusOK http code.  It is shorthand for Reply(ctx, w, response, http.StatusOK).
	Ok(ctx context.Context, w http.ResponseWriter, response interface{})
	// Err formats error response based on passed message
	Err(ctx context.Context, w http.ResponseWriter, errmsg string, status int)
}

type JSONReplier struct{}

func (JSONReplier) reply(ctx context.Context, w http.ResponseWriter, response io.Reader, status int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)

	io.Copy(w, response)
}

func (rpl JSONReplier) Reply(ctx context.Context, w http.ResponseWriter, response interface{}, status int) {
	// check reader
	if reader, ok := response.(io.Reader); ok {
		rpl.reply(ctx, w, reader, status)
		return
	}

	// marshal body
	data, err := json.Marshal(response)
	if err != nil {
		log.FromContext(ctx).Error("unable marshal response", "err", err)
		rpl.Err(ctx, w, "internal error", http.StatusInternalServerError)
		return
	}

	//create reader
	reader := bytes.NewBuffer(data)
	rpl.reply(ctx, w, reader, status)
}

func (rpl JSONReplier) Ok(ctx context.Context, w http.ResponseWriter, response interface{}) {
	rpl.Reply(ctx, w, response, http.StatusOK)
}

func (rpl JSONReplier) Err(ctx context.Context, w http.ResponseWriter, errmsg string, status int) {
	apierr := struct {
		Message string `json:"message"`
	}{errmsg}

	data, _ := json.Marshal(apierr)
	reader := bytes.NewBuffer(data)
	rpl.reply(ctx, w, reader, status)
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
