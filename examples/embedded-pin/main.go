package main

import (
	"log"
	"net/http"

	"github.com/frey788/heimdall"
)

func main() {
	h, err := heimdall.Install(heimdall.InstallOptions{
		DashboardPath: "ops/heimdall",
		PINEnabled:    true,
		PIN:           "1234",
	})
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", h.HTTPMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("embedded pin example"))
	})))

	if err := h.Mount(mux); err != nil {
		log.Fatal(err)
	}

	log.Println("embedded-pin example listening on :8080")
	log.Println("dashboard path: /ops/heimdall/")
	log.Println("use header X-Heimdall-PIN: 1234 for dashboard requests")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
