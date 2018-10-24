package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// Reply is the function that send response to the user,
// it is invoked internally from middlewares.
// It is represented as a variable, so one could replace it with custom function.
var Reply = func(ctx context.Context, w http.ResponseWriter, response io.Reader, status int) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(status)

	io.Copy(w, response)
}

// ReplyOk is the function that format and send good json response to the user,
// it is invoked internally from middlewares.
// It is represented as a variable, so one could replace it with custom function.
var ReplyOk = func(ctx context.Context, w http.ResponseWriter, response interface{}) {
	data, err := json.Marshal(response)
	if err != nil {
		ReplyError(ctx, w, "unable marshal response: "+err.Error(), http.StatusInternalServerError)
		return
	}
	resp := bytes.NewBuffer(data)
	Reply(ctx, w, resp, http.StatusOK)
}

// ReplyError is the function that format and send good bad response to the user,
// it is invoked internally from middlewares.
// It is represented as a variable, so one could replace it with custom function.
var ReplyError = func(ctx context.Context, w http.ResponseWriter, errmsg string, status int) {
	apierr := struct {
		Message string `json:"message"`
	}{errmsg}

	data, _ := json.Marshal(apierr)
	resp := bytes.NewBuffer(data)
	Reply(ctx, w, resp, status)
}
