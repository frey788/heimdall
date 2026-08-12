package main

import (
	"log"
	"net/http"

	"github.com/frey788/heimdall"
)

func main() {
	h, err := heimdall.Install(heimdall.InstallOptions{
		DashboardPath: "_heimdall",
	})
	if err != nil {
		log.Fatal(err)
	}

	app := h.HTTPMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/ping":
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte("pong"))
		default:
			http.NotFound(rw, req)
		}
	}))

	mux := http.NewServeMux()
	mux.Handle("/", app)

	if err := h.Mount(mux); err != nil {
		log.Fatal(err)
	}

	log.Println("http-basic example listening on :8080")
	log.Println("dashboard: http://localhost:8080/_heimdall/")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
