package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/hashicorp/nomad/demo/grpc-checks/example"
	"google.golang.org/grpc"
	ghc "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {

	port := os.Getenv("GRPC_HC_PORT")
	if port == "" {
		port = "3333"
	}
	address := fmt.Sprintf(":%s", port)

	log.Printf("creating tcp listener on %s", address)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Printf("unable to create listener: %v", err)
		os.Exit(1)
	}

	log.Printf("creating grpc server")
	grpcServer := grpc.NewServer()

	log.Printf("registering health server")
	ghc.RegisterHealthServer(grpcServer, example.New())

	log.Printf("listening ...")
	if err := grpcServer.Serve(listener); err != nil {
		log.Printf("unable to listen: %v", err)
		os.Exit(1)
	}
}
