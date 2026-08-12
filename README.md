# heimdall

## Quick start

```go
package main

import (
	"log"
	"net/http"

	"github.com/frey788/heimdall"
)

func main() {
	h, err := heimdall.Install(heimdall.InstallOptions{
		DashboardPath: "_heimdall",
		PINEnabled:    true,
		PIN:           "1234",
	})
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", h.HTTPMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("ok"))
	})))

	if err := h.Mount(mux); err != nil {
		log.Fatal(err)
	}

	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

Heimdall runs in embedded mode by default, mounted at your configured path on the same app server.
If PIN protection is enabled, send `X-Heimdall-PIN` to access dashboard endpoints.

## Docker development container

The repository includes a development container with Go and gRPC tooling preinstalled.

Included tools:
- Go 1.22
- `protoc` (protobuf compiler)
- `protoc-gen-go`
- `protoc-gen-go-grpc`

Build and start the container:

```bash
docker compose up -d --build
docker compose exec heimdall-dev bash
```

Inside container, run checks:

```bash
go test ./...
```

## Runtime container template

For production applications that consume Heimdall, use [Dockerfile.runtime.example](Dockerfile.runtime.example).

This is separate from the development container and produces a small runtime image.

Example build:

```bash
docker build -f Dockerfile.runtime.example -t my-app:latest --build-arg APP_CMD=./cmd/server .
```

## Usage examples

1. net/http integration
- [examples/http-basic/main.go](examples/http-basic/main.go)

2. gRPC server and client interceptors
- [examples/grpc-interceptors/main.go](examples/grpc-interceptors/main.go)

3. Embedded dashboard path with PIN
- [examples/embedded-pin/main.go](examples/embedded-pin/main.go)

## CI workflow

GitHub Actions CI is defined in [.github/workflows/ci.yml](.github/workflows/ci.yml) and runs on push and pull request with:

1. gofmt format check
2. go vet
3. go test ./...