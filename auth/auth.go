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

type NextHandlerBuilder func(uid string, freebie bool) http.Handler

// Claims is set of values transferred by jwt
type Claims struct {
	jwt.StandardClaims
	UID     string `json:"uid,omitempty"`
	Freebie byte   `json:"frb,omitempty"` // 0 or 1
}

// AuthenticationHandler receives receipt and verifies it. Uses receipt for authenticate and authorize the user.
// If successfully returns access token
func AuthenticationHandler(secret string, period time.Duration, rs iap.ReceiptService, knownBundles []string, trustedDevices []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		expireToken := time.Now().Add(period)

		scope, bundleID, idForVendor, receipt, errmsg := AuthParams(r)
		if errmsg != "" {
			reply.Err(ctx, w, http.StatusBadRequest, errmsg)
		}

		ctx = usage.NewContext(ctx,
			"scope", scope,
			"bundle_id", bundleID,
			"device_id", idForVendor,
			"receipt_len", len(receipt),
		)

		// check if request is made on behalf of known app
		if len(knownBundles) > 0 && !stringInSlice(bundleID, knownBundles) {
			reply.Err(ctx, w, http.StatusForbidden, "unregistered bundle")
			return
		}

		if strings.Contains(scope, "limited") {
			user := []byte(idForVendor)
			ReplyJWT(ctx, w, secret, expireToken, user, 1)
			return
		}

		// check if it's trusted device, and no receipt is needed
		if len(trustedDevices) > 0 && stringInSlice(idForVendor, trustedDevices) {
			user := []byte(idForVendor)
			ReplyJWT(ctx, w, secret, expireToken, user, 0)
			return
		}

		// check if receipt valid, just check length
		if len(receipt) == 0 {
			reply.Err(ctx, w, http.StatusBadRequest, "please provide correct receipt")
			return
		}

		expireSubscription, user, err := AnySubscription(ctx, rs, receipt)
		if err != nil {
			// it's bad practice to expose internal errors, just log it.
			log.Error(ctx, errmsg, "err", err, "type", "auth.iap")
			reply.Err(ctx, w, http.StatusInternalServerError, http.StatusText(http.StatusInternalServerError))
			return
		}
		if expireSubscription.IsZero() {
			reply.Err(ctx, w, http.StatusForbidden, "no active subscriptions")
			return
		}

		// set token expire date no more than subscription expiration.
		if expireToken.After(expireSubscription) {
			expireToken = expireSubscription
		}

		ReplyJWT(ctx, w, secret, expireToken, user, 0)
	}
}

func AuthParams(r *http.Request) (scope, bundleID, idForVendor string, receipt []byte, string string) {
	// check if we have any posted parameters
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return scope, bundleID, idForVendor, receipt, "unable to find posted parameters: " + err.Error()
	}

	scope = r.FormValue("scope")

	bundleID = r.FormValue("bundle_id")
	if bundleID == "" {
		return scope, bundleID, idForVendor, receipt, "please provide correct bundle_id"
	}

	idForVendor = r.FormValue("identifier_for_vendor")
	if idForVendor == "" {
		return scope, bundleID, idForVendor, receipt, "please provide correct identifier_for_vendor"
	}

	receipt, errmsg := readReceipt(r)
	return scope, bundleID, idForVendor, receipt, errmsg
}

// AnySubscription check if user has any paid subscription.
// BUt in general you could have more than one auto-renewable subscription.
func AnySubscription(ctx context.Context, rs iap.ReceiptService, receipt []byte) (time.Time, []byte, error) {
	// get all subscriptions, including expired (not sure about canceled)
	subscriptions, err := rs.GetAutoRenewableIAPs(ctx, receipt, iap.ARActive|iap.ARFree)
	if err != nil {
		return time.Time{}, nil, err
	}
	if len(subscriptions) == 0 {
		return time.Time{}, nil, err
	}

	sbs := subscriptions[0]
	// set token expire date no more than subscription expiration.
	expireSubscription := sbs.SubscriptionExpirationDate.Time

	// calculate user id:
	//  - use OriginalTransactionID as base for user id
	//  - if your API allow free users (without IAP), you could use identifierForVendor (aka device id)
	// todo: clarify uncertainty:
	//  1) OriginalTransactionID may not be unique if user has canceled purchase. Solution - add OriginalPurchaseDate (simple)
	//  2) OriginalTransactionID may not be unique across multiple devices (or even behave like identifierForVendor ). Solution - involve WebOrderLineItemID (hard)
	user := sha256.Sum224([]byte(sbs.OriginalTransactionID + sbs.OriginalPurchaseDate.String()))

	return expireSubscription, user[:], nil
}

func ReplyJWT(ctx context.Context, w http.ResponseWriter, secret string, expireToken time.Time, user []byte, freebie byte) {
	// write claims: token body
	claims := Claims{
		UID:     base64.RawStdEncoding.EncodeToString(user),
		Freebie: freebie,
	}
	claims.ExpiresAt = expireToken.Unix()
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

	scope := "all"
	if freebie != 0 {
		scope = "limited"
	}

	expSec := time.Since(expireToken).Seconds()
	response := map[string]interface{}{
		"access_token": tokenString,
		"token_type":   "Bearer",
		"expires_in":   -int(expSec),
		"scope":        scope,
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
			"freebie", claims.Freebie,
		)

		next(claims.UID, claims.Freebie == 1).ServeHTTP(w, r.WithContext(ctx))
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
