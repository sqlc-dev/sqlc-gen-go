package main

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/sqlc-dev/plugin-sdk-go/codegen"
	pb "github.com/sqlc-dev/plugin-sdk-go/plugin"
	"google.golang.org/grpc"

	golang "github.com/sqlc-dev/sqlc-gen-go/internal"
)

func main() {
	listenAddr := os.Getenv("LISTEN_ADDR")
	sock, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalln("listening failed:", err)
	}

	codegenServer := &server{handler: golang.Generate}
	grpcServer := grpc.NewServer()
	pb.RegisterCodegenServiceServer(grpcServer, codegenServer)

	if err := grpcServer.Serve(sock); err != nil {
		log.Fatalln("serving gRPC failed:", err)
	}
}

type server struct {
	pb.UnimplementedCodegenServiceServer

	handler codegen.Handler
}

func (s *server) Generate(ctx context.Context, req *pb.GenerateRequest) (*pb.GenerateResponse, error) {
	return s.handler(ctx, req)
}
