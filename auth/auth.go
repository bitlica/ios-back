package auth

import (
	"context"
	"crypto/sha256"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Loofort/ios-back/iap"
	"github.com/dgrijalva/jwt-go"
)

// AuthenticationHandler receives receipt and verifies it. Uses receipt for authenticate and authorize the user.
// If successfully returns access token
func AuthenticationHandler(secret []byte, period time.Duration, rs iap.ReceiptService, knownBundles ...string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		bundleID, deviceID, receipt, errmsg := authParams(r)
		if errmsg != "" {
			ReplyError(ctx, w, errmsg, http.StatusBadRequest)
			return
		}

		// validate bundle id
		if len(knownBundles) > 1 && !stringInSlice(bundleID, knownBundles) {
			ReplyError(ctx, w, "unregistered bundle", http.StatusForbidden)
			return
		}

		// get active or free subscription, no expired or canceled can be returned by following method.
		subscriptions, err := rs.GetAutoRenewableIAPs(ctx, receipt)
		if err != nil {
			// remember it's bad practice to expose internal errors.
			// we doing this only for example purposes.
			errmsg := "unexpected problem during receipt verifying: " + err.Error()
			ReplyError(ctx, w, errmsg, http.StatusInternalServerError)
			return
		}
		if len(subscriptions) == 0 {
			ReplyError(ctx, w, "no active subscriptions", http.StatusForbidden)
			return
		}

		// in general you could have more than one auto-renewable subscription.
		// but in this middleware we assume it's only one.
		sbs := subscriptions[0]
		expireSubscription := sbs.SubscriptionExpirationDate.Time

		// set token expire date no more than subscription expiration.
		expireToken := time.Now().Add(period)
		if expireToken.After(expireSubscription) {
			expireToken = expireSubscription
		}

		// calculate user id:
		// use OriginalTransactionID as base for user id
		// todo:
		// clarify uncertainty:
		// 1) OriginalTransactionID may not be unique if user has canceled purchase. Solution - add OriginalPurchaseDate (simple)
		// 2) OriginalTransactionID may not be unique across multiple devices (or even behave like identifierForVendor ). Solution - involve WebOrderLineItemID (hard)
		userID := sha256.Sum256([]byte(sbs.OriginalTransactionID + sbs.OriginalPurchaseDate.String()))

		// write claims: token body
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"userid":  userID,
			"enddate": expireSubscription,
			"expire":  expireToken,
		})

		// Sign and get the complete encoded token as a string using the secret
		tokenString, err := token.SignedString(secret)
		if err != nil {
			// remember it's bad practice to expose internal errors.
			// we doing this only for example purposes.
			errmsg := "unable to create auth token: " + err.Error()
			ReplyError(ctx, w, errmsg, http.StatusInternalServerError)
			return
		}

		response := map[string]string{
			"access_token": tokenString,
			"token_type":   "Bearer",
			"expire_date":  expireToken.String(),
		}
		ReplyOk(ctx, w, response)
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

// this is so widely used function.
// wonder why it's not in std lib yet.
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// IntrospectHandler verifies access token.
// It forbids or requests authorization if token is invalid.
func IntrospectHandler(handler http.Handler, secret string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		tokenString, errmsg := introParams(r)
		if errmsg != "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			ReplyError(ctx, w, errmsg, http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return secret, nil
		})
		if err != nil {
			ReplyError(ctx, w, "invalid access token", http.StatusForbidden)
			return
		}

		ctx = context.WithValue(ctx, "token", token)
		r = r.WithContext(ctx)
		handler.ServeHTTP(w, r)
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

type AuthInfo struct {
}
