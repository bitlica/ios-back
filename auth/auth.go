package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/Loofort/ios-back/iap"
	"github.com/Loofort/ios-back/reply"
	"github.com/dgrijalva/jwt-go"
)

// Claims is set of values transferred by jwt
type Claims struct {
	jwt.StandardClaims
	User string `json:"usr,omitempty"`
}

// AuthenticationHandler receives receipt and verifies it. Uses receipt for authenticate and authorize the user.
// If successfully returns access token
func AuthenticationHandler(secret string, period time.Duration, rs iap.ReceiptService, knownBundles ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		bundleID, receipt, errmsg := authParams(r)
		if errmsg != "" {
			reply.FromContext(ctx).Err(ctx, w, errmsg, http.StatusBadRequest)
			return
		}

		// validate bundle id
		if len(knownBundles) > 1 && !stringInSlice(bundleID, knownBundles) {
			reply.FromContext(ctx).Err(ctx, w, "unregistered bundle", http.StatusForbidden)
			return
		}

		// get all subscriptions, including expired (not sure about canceled)
		subscriptions, err := rs.GetAutoRenewableIAPs(ctx, receipt)
		if err != nil {
			// remember it's bad practice to expose internal errors.
			// we doing this only for example purposes.
			errmsg := "unexpected problem during receipt verifying: " + err.Error()
			reply.FromContext(ctx).Err(ctx, w, errmsg, http.StatusInternalServerError)
			return
		}
		var active []iap.AutoRenewable
		for _, sbs := range subscriptions {
			if sbs.State == iap.ARActive || sbs.State == iap.ARFree {
				active = append(active, sbs)
			}
		}
		if len(active) == 0 {
			reply.FromContext(ctx).Err(ctx, w, "no active subscriptions", http.StatusForbidden)
			return
		}

		// in general you could have more than one auto-renewable subscription.
		// but in this middleware we assume it's only one.
		sbs := active[0]
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

		// write claims: token body
		claims := Claims{}
		claims.ExpiresAt = expireToken.Unix()
		claims.User = base64.RawStdEncoding.EncodeToString(user[:])
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

		// Sign and get the complete encoded token as a string using the secret
		tokenString, err := token.SignedString([]byte(secret))
		if err != nil {
			// remember it's bad practice to expose internal errors.
			// we doing this only for example purposes.
			errmsg := "unable to create auth token: " + err.Error()
			reply.FromContext(ctx).Err(ctx, w, errmsg, http.StatusInternalServerError)
			return
		}

		expSec := time.Since(expireToken).Seconds()
		response := map[string]interface{}{
			"access_token": tokenString,
			"token_type":   "Bearer",
			"expires_in":   -int(expSec),
		}
		reply.FromContext(ctx).Ok(ctx, w, response)
	}
}

func authParams(r *http.Request) (bundleID string, receipt []byte, errmsg string) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return "", nil, err.Error()
	}

	bundleID = r.FormValue("bundle_id")
	if bundleID == "" {
		return "", nil, "please provide correct bundle_id"
	}

	fr, _, err := r.FormFile("receipt")
	if err != nil {
		return "", nil, "unable to read receipt: " + err.Error()
	}

	receipt, err = ioutil.ReadAll(fr)
	if err != nil {
		return "", nil, "unable to read receipt: " + err.Error()
	}

	if len(receipt) == 0 {
		return "", nil, "please provide correct receipt"
	}

	return bundleID, receipt, ""
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
func IntrospectHandler(secret string, handler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		tokenString, errmsg := introParams(r)
		if errmsg != "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			reply.FromContext(ctx).Err(ctx, w, errmsg, http.StatusUnauthorized)
			return
		}

		claims := Claims{}
		keyFunc := func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		}
		_, err := jwt.ParseWithClaims(tokenString, &claims, keyFunc)
		if err != nil {
			reply.FromContext(ctx).Err(ctx, w, "invalid access token:"+err.Error(), http.StatusUnauthorized)
			return
		}

		// now we have claims object with user id.
		// What to do with this depends on your business logic.
		// At minimum you may want to add it to you log records.
		// Or you may want to pass it to other middleware for performing some logic - however, avoid to use context for this kind of propagation.

		handler.ServeHTTP(w, r)
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
