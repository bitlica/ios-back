package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Loofort/ios-back/auth"
	"github.com/Loofort/ios-back/iap"
)

const (
	jwtSecret = "jwt secret"
	jwtPeriod = time.Hour
	bundleID  = "com.myfirm.myapp"
)

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
	timeHandler := auth.IntrospectHandler(jwtSecret, http.HandlerFunc(helloAPI))

	mux := &http.ServeMux{}
	mux.Handle("/token", authHandler)
	mux.Handle("/hello", timeHandler)

	return mux
}

func helloAPI(w http.ResponseWriter, r *http.Request) {
	// enter here only if access token was valied

	name := r.FormValue("name")
	if name == "" {
		name = "world"
	}
	response := map[string]string{
		"hello": name,
	}

	// ReplyOk is just a convenient helper to format json response.
	// it is not necessary to use exactly it.
	auth.ReplyOk(r.Context(), w, response)
}
