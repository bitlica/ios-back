package iap

import (
	"context"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Loofort/ios-back/iap"
	jwt "github.com/dgrijalva/jwt-go"
)

type Reply func(ctx context.Context, status int, response interface{}, w http.ResponseWriter)

func AuthenticationHandler(secret []byte, period time.Duration, rs iap.ReceiptService, reply Reply) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		bundleID := r.FormValue("bundle_id")
		if bundleID == "" {
			msg := "please provide correct bundle_id"
			reply(ctx, http.StatusBadRequest, msg, w)
			return
		}

		deviceID := r.FormValue("identifier_for_vendor")
		if deviceID == "" {
			msg := "please provide correct identifier_for_vendor"
			reply(ctx, http.StatusBadRequest, msg, w)
			return
		}

		fr, _, err := r.FormFile("receipt")
		if err != nil {
			msg := "unable to read receipt: " + err.Error()
			reply(ctx, http.StatusBadRequest, msg, w)
			return
		}

		b, err := ioutil.ReadAll(fr)
		if err != nil {
			msg := "unable to read receipt: " + err.Error()
			reply(ctx, http.StatusBadRequest, msg, w)
			return
		}

		if len(b) == 0 {
			msg := "please provide correct receipt"
			reply(ctx, http.StatusBadRequest, msg, w)
			return
		}

		subscriptions, err := rs.GetAutoRenewableIAPs(ctx, b)
		if err != nil {
			// remember it's bad practice to expose internal errors.
			// we doing this only for example purposes.
			msg := "unexpected problem during receipt verifying: " + err.Error()
			reply(ctx, http.StatusInternalServerError, msg, w)
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

		// create Auth structure

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"bundleid": bundleID,
			"deviceid": deviceID,
			"enddate":  endDate,
			"expire":   Expire,
		})

		// Sign and get the complete encoded token as a string using the secret
		tokenString, err := token.SignedString(secret)
		if err != nil {
			// remember it's bad practice to expose internal errors.
			// we doing this only for example purposes.
			msg := "unable to create auth token: " + err.Error()
			reply(ctx, http.StatusInternalServerError, msg, w)
			return
		}

		response := map[string]string{
			"access_token": tokenString,
			"token_type":   "Bearer",
			"expire_date":  Expire,
		}
		reply(ctx, http.StatusOK, response, w)
	})
}

func Introspect(w http.ResponseWriter, r *http.Request) {
}
