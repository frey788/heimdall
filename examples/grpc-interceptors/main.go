package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/frey788/heimdall"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	h, err := heimdall.Install(heimdall.InstallOptions{
		DashboardPath: "_heimdall",
	})
	if err != nil {
		log.Fatal(err)
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(h.GRPCUnaryServerInterceptor()),
		grpc.ChainStreamInterceptor(h.GRPCStreamServerInterceptor()),
	)

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	go func() {
		log.Println("grpc-interceptors example gRPC server on :50051")
		if serveErr := grpcServer.Serve(lis); serveErr != nil {
			log.Fatalf("grpc server error: %v", serveErr)
		}
	}()

	go func() {
		time.Sleep(300 * time.Millisecond)
		conn, dialErr := grpc.DialContext(
			context.Background(),
			"127.0.0.1:50051",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithChainUnaryInterceptor(h.GRPCUnaryClientInterceptor()),
			grpc.WithChainStreamInterceptor(h.GRPCStreamClientInterceptor()),
		)
		if dialErr != nil {
			log.Printf("dial error: %v", dialErr)
			return
		}
		defer conn.Close()

		client := healthpb.NewHealthClient(conn)
		_, callErr := client.Check(context.Background(), &healthpb.HealthCheckRequest{})
		if callErr != nil {
			log.Printf("health check call error: %v", callErr)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/", h.HTTPMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("grpc example running"))
	})))

	if err := h.Mount(mux); err != nil {
		log.Fatal(err)
	}

	log.Println("dashboard: http://localhost:8080/_heimdall/")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
