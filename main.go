package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Loofort/ios-back/auth"
	"github.com/Loofort/ios-back/iap"
	ilog "github.com/Loofort/ios-back/log"
	"github.com/Loofort/ios-back/mw"
	"github.com/Loofort/ios-back/reply"
)

const (
	jwtSecret = "jwt secret"
	jwtPeriod = time.Hour
	bundleID  = "com.myfirm.myapp"
)

func init() {
	initLog()
}

func main() {
	if len(os.Args) == 1 {
		log.Fatalf("secret is missed, usage:\n%v secret", os.Args[0])
	}

	rs := iap.ReceiptService{Secret: os.Args[1]}
	servemux := serveMux(rs)
	log.Fatalln(http.ListenAndServe(":8080", servemux))
}

func serveMux(rs iap.ReceiptService) *http.ServeMux {
	authHandler := auth.AuthenticationHandler(jwtSecret, jwtPeriod, rs, bundleID)
	apiHandler := auth.IntrospectHandler(jwtSecret, newUserHandler)

	mux := &http.ServeMux{}
	mux.Handle("/token", mw.NewCommonHandler(authHandler))
	mux.Handle("/user", mw.NewCommonHandler(apiHandler))

	return mux
}

/*******************  define how to log ********************/
func initLog() {
	ilog.New = func() ilog.Logger {
		return myLogger{}
	}
}

type myLogger struct {
	with string
}

func (l myLogger) Info(message string, keyValues ...interface{}) {
	kv := splitKV(keyValues)
	fmt.Printf("%s: %s%s\n", message, l.with, kv)
}
func (l myLogger) Error(message string, keyValues ...interface{}) {
	kv := splitKV(keyValues)
	log.Printf("%s: %s%s\n", message, l.with, kv)
}
func (l myLogger) With(keyValues ...interface{}) ilog.Logger {
	l.with += splitKV(keyValues)
	return l
}

func splitKV(keyValues []interface{}) string {
	if len(keyValues)%2 != 0 {
		keyValues = append(keyValues, "nil")
	}
	str := ""
	for i := 0; i < len(keyValues); i += 2 {
		str += fmt.Sprintf("%s=%v, ", keyValues[i], keyValues[i+1])
	}
	return str
}

/*************************** user API ***************************/

type userAPI struct {
	UID string `json:"uid"`
}

func newUserHandler(uid string) http.Handler {
	return userAPI{uid}
}

func (api userAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// enter here only if access token was valied
	ctx := r.Context()

	// the response is the handler object itself

	// reply.Ok is just a convenient helper to format json response.
	// it is not necessary to use exactly it.
	reply.Ok(ctx, w, api)
}
