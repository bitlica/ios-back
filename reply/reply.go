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

// FormatErr formats error response body based on passed message
var FormatErr = func(errmsg string) io.Reader {
	apierr := struct {
		Message string `json:"message"`
	}{errmsg}

	data, _ := json.Marshal(apierr) // can't be error
	return bytes.NewBuffer(data)
}

// FormatOk formats success response body
var FormatOk = func(response interface{}) (io.Reader, error) {
	if reader, ok := response.(io.Reader); ok {
		return reader, nil
	}

	data, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewBuffer(data)
	return reader, nil
}

// Err is helper function. It formats error and call the replier from context
func Err(ctx context.Context, w http.ResponseWriter, status int, errmsg string) {
	reader := FormatErr(errmsg)
	FromContext(ctx).Reply(ctx, w, status, reader)
}

// Err is helper function. It formats success respons and call replier from context
func Ok(ctx context.Context, w http.ResponseWriter, response interface{}) {
	reader, err := FormatOk(response)
	if err != nil {
		log.FromContext(ctx).Error("unable marshal response", "err", err, "type", "reply")
		Err(ctx, w, http.StatusInternalServerError, "internal error")
		return
	}

	FromContext(ctx).Reply(ctx, w, http.StatusOK, reader)
}

// Replier sends response to the user.
type Replier interface {
	Reply(ctx context.Context, w http.ResponseWriter, status int, response io.Reader)
}

type JSONReplier struct{}

func (JSONReplier) Reply(ctx context.Context, w http.ResponseWriter, status int, response io.Reader) {
	logger := log.FromContext(ctx)

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)

	_, err := io.Copy(w, response)
	if err != nil {
		logger.Error("unable to reply", "err", err, "type", "conn")
	}
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
