package iap

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Loofort/ios-back/iap"
	"github.com/dgrijalva/jwt-go"
)

type Reply func(ctx context.Context, w http.ResponseWriter, response io.Reader, status int)

// helper function for marshaling good json responses
func APIOK(response interface{}) (io.Reader, int) {
	data, err := json.Marshal(response)
	if err != nil {
		return APIError("unable marshal response: "+err.Error(), http.StatusInternalServerError)
	}
	resp := bytes.NewBuffer(data)
	return resp, http.StatusOK
}

// helper function for marshaling bad json responses
func APIError(errmsg string, status int) (io.Reader, int) {
	apierr := struct {
		Message string `json:"message"`
	}{errmsg}

	data, _ := json.Marshal(apierr)
	resp := bytes.NewBuffer(data)
	return resp, status
}

// AuthenticationHandler receives receipt and verifies it. Uses receipt for authenticate and authorize the user.
// If successfully returns access token
func AuthenticationHandler(reply Reply, secret []byte, period time.Duration, rs iap.ReceiptService) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		bundleID, deviceID, receipt, errmsg := authParams(r)
		if errmsg != "" {
			reply(ctx, w, APIError(errmsg, http.StatusBadRequest))
			return
		}

		subscriptions, err := rs.GetAutoRenewableIAPs(ctx, receipt)
		if err != nil {
			// remember it's bad practice to expose internal errors.
			// we doing this only for example purposes.
			msg := "unexpected problem during receipt verifying: " + err.Error()
			reply(ctx, w, APIError(errmsg, http.StatusInternalServerError))
			return
		}

		// do some business logic.
		// Assume we could have one active auto-renewable subscription.
		var endDate time.Time
		for _, sbs := range subscriptions {
			if sbs.State != iap.AROk {
				continue
			}
			endDate = sbs.SubscriptionExpirationDate.Time
		}

		// set token expire date no more than subscription expiration.
		expire := time.Now().Add(period)
		if expire.After(endDate) {
			endDate = expire
		}

		// write claims: token body
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"bundleid": bundleID,
			"deviceid": deviceID,
			"enddate":  endDate,
			"expire":   expire,
		})

		// Sign and get the complete encoded token as a string using the secret
		tokenString, err := token.SignedString(secret)
		if err != nil {
			// remember it's bad practice to expose internal errors.
			// we doing this only for example purposes.
			msg := "unable to create auth token: " + err.Error()
			reply(ctx, w, APIError(errmsg, http.StatusInternalServerError))
			return
		}

		response := map[string]string{
			"access_token": tokenString,
			"token_type":   "Bearer",
			"expire_date":  Expire,
		}
		reply(ctx, w, APIOK(response))
	})
}

func authParams(r *http.Request) (bundleID, deviceID string, receipt []byte, errmsg string) {

	bundleID = r.FormValue("bundle_id")
	if bundleID == "" {
		return "", "", nil, "please provide correct bundle_id"
	}

	deviceID = r.FormValue("identifier_for_vendor")
	if deviceID == "" {
		return "", "", nil, "please provide correct identifier_for_vendor"
	}

	fr, _, err := r.FormFile("receipt")
	if err != nil {
		return "", "", nil, "unable to read receipt: " + err.Error()
	}

	receipt, err = ioutil.ReadAll(fr)
	if err != nil {
		return "", "", nil, "unable to read receipt: " + err.Error()
	}

	if len(receipt) == 0 {
		return "", "", nil, "please provide correct receipt"
	}

	return bundleID, deviceID, receipt, ""
}

// IntrospectHandler verifies access token.
// It forbids or requests authorization if token is invalid.
func IntrospectHandler(reply Reply, handler http.Handler, secret string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		tokenString, errmsg := introParams(r)
		if errmsg != "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			reply(ctx, w, APIError(errmsg, http.StatusUnauthorized))
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return secret, nil
		})
		if err != nil {
			reply(ctx, w, "invalid access token", http.StatusForbidden)
			return
		}

		ctx = context.WithValue(ctx, "token", token)
		r = r.WithContext(ctx)
		handler(w, r)
	})
}

func introParams(r *http.Request) (token, errmsg string) {
	bearer := r.Header.Get("Authorization")
	if bearer == "" {
		return "", "Authorization header is missing"
	}

	prefix := "Bearer "
	if !strings.HasPrefix(bearer, prefix) {
		return "", "only 'Bearer' authorization token is supported"
	}

	return strings.TrimPrefix(bearer, prefix), ""
}
