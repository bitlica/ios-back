package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Loofort/ios-back/iap"
	"github.com/Loofort/ios-back/log"
	"github.com/Loofort/ios-back/reply"
	"github.com/Loofort/ios-back/usage"
	"github.com/dgrijalva/jwt-go"
)

type NextHandlerBuilder func(uid string) http.Handler

// Claims is set of values transferred by jwt
type Claims struct {
	jwt.StandardClaims
	UID string `json:"uid,omitempty"`
}

// AuthenticationHandler receives receipt and verifies it. Uses receipt for authenticate and authorize the user.
// If successfully returns access token
func AuthenticationHandler(secret string, period time.Duration, rs iap.ReceiptService, knownBundles []string, trustedDevices []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// check if we have any posted parameters
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			reply.Err(ctx, w, http.StatusBadRequest, "unable to find posted parameters: "+err.Error())
		}

		bundleID := r.FormValue("bundle_id")
		if bundleID == "" {
			reply.Err(ctx, w, http.StatusBadRequest, "please provide correct bundle_id")
		}
		ctx = usage.NewContext(ctx,
			"bundle_id", bundleID,
		)

		// check if request is made on behalf of known app
		if len(knownBundles) > 1 && !stringInSlice(bundleID, knownBundles) {
			reply.Err(ctx, w, http.StatusForbidden, "unregistered bundle")
			return
		}

		idForVendor := r.FormValue("identifier_for_vendor")
		if idForVendor == "" {
			reply.Err(ctx, w, http.StatusBadRequest, "please provide correct identifier_for_vendor")
			return
		}
		ctx = usage.NewContext(ctx,
			"device_id", idForVendor,
		)

		// check if it's trusted device, and no receipt is needed
		if len(trustedDevices) > 1 && stringInSlice(idForVendor, trustedDevices) {
			ctx = usage.NewContext(ctx,
				"trusted", true,
			)
			expireToken := time.Now().Add(period)
			user := []byte(idForVendor)
			ReplyJWT(ctx, w, secret, expireToken, user)
			return
		}

		receipt, errmsg := readReceipt(r)
		if errmsg != "" {
			reply.Err(ctx, w, http.StatusBadRequest, errmsg)
			return
		}
		ctx = usage.NewContext(ctx,
			"receipt_len", len(receipt),
		)

		// check if receipt valid, just check length
		if len(receipt) == 0 {
			reply.Err(ctx, w, http.StatusBadRequest, "please provide correct receipt")
			return
		}

		// get all subscriptions, including expired (not sure about canceled)
		subscriptions, err := rs.GetAutoRenewableIAPs(ctx, receipt, iap.ARActive|iap.ARFree)
		if err != nil {
			errmsg := "unexpected problem during receipt verifying"
			// remember it's bad practice to expose internal errors.
			// we doing this only for example purposes.
			log.Error(ctx, errmsg, "err", err, "type", "auth.iap")
			reply.Err(ctx, w, http.StatusInternalServerError, errmsg)
			return
		}
		if len(subscriptions) == 0 {
			reply.Err(ctx, w, http.StatusForbidden, "no active subscriptions")
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
		//  - use OriginalTransactionID as base for user id
		//  - if your API allow free users (without IAP), you could use identifierForVendor (aka device id)
		// todo: clarify uncertainty:
		//  1) OriginalTransactionID may not be unique if user has canceled purchase. Solution - add OriginalPurchaseDate (simple)
		//  2) OriginalTransactionID may not be unique across multiple devices (or even behave like identifierForVendor ). Solution - involve WebOrderLineItemID (hard)
		user := sha256.Sum224([]byte(sbs.OriginalTransactionID + sbs.OriginalPurchaseDate.String()))

		ReplyJWT(ctx, w, secret, expireToken, user[:])
	}
}

func ReplyJWT(ctx context.Context, w http.ResponseWriter, secret string, expireToken time.Time, user []byte) {
	// write claims: token body
	claims := Claims{}
	claims.ExpiresAt = expireToken.Unix()
	claims.UID = base64.RawStdEncoding.EncodeToString(user)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign and get the complete encoded token as a string using the secret
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		errmsg := "unable to create auth token"
		// remember it's bad practice to expose internal errors.
		// we doing this only for example purposes.
		log.Error(ctx, errmsg, "err", err, "type", "auth.jwt")
		reply.Err(ctx, w, http.StatusInternalServerError, errmsg)
		return
	}

	expSec := time.Since(expireToken).Seconds()
	response := map[string]interface{}{
		"access_token": tokenString,
		"token_type":   "Bearer",
		"expires_in":   -int(expSec),
	}

	// add usage for log info purposes
	ctx = usage.NewContext(ctx,
		"uid", claims.UID,
		"expires_in", -int(expSec),
	)
	reply.Ok(ctx, w, response)
}

func readReceipt(r *http.Request) (receipt []byte, errmsg string) {
	fr, _, err := r.FormFile("receipt")
	if err != nil {
		return nil, "unable to read receipt: " + err.Error()
	}

	receipt, err = ioutil.ReadAll(fr)
	if err != nil {
		return nil, "unable to read receipt: " + err.Error()
	}

	return receipt, ""
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
func IntrospectHandler(secret string, next NextHandlerBuilder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		tokenString, errmsg := introParams(r)
		if errmsg != "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			reply.Err(ctx, w, http.StatusUnauthorized, errmsg)
			return
		}

		claims := Claims{}
		keyFunc := func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		}
		_, err := jwt.ParseWithClaims(tokenString, &claims, keyFunc)
		if err != nil {
			errmsg = "token expired"
			if verr, ok := err.(*jwt.ValidationError); !ok || verr.Errors&jwt.ValidationErrorExpired == 0 {
				errmsg = "invalid access token"
				// log system error or hacker attack
				log.Error(ctx, "invalid access token", "err", err, "type", "auth.invalid")
			}

			w.Header().Set("WWW-Authenticate", "Bearer")
			reply.Err(ctx, w, http.StatusUnauthorized, errmsg)
			return
		}

		// now we have claims object with user id.
		// What to do with this depends on your business logic.
		// At minimum you may want to add it to you log records.
		// Or you may want to pass it to other middleware for performing some logic - however, avoid to use context for this kind of propagation.
		ctx = usage.NewContext(ctx,
			"uid", claims.UID,
		)

		next(claims.UID).ServeHTTP(w, r.WithContext(ctx))
	}
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
