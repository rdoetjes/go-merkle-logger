package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"phonax.com/merkle/internal/server"
	"phonax.com/merkle/proto"
)

func main() {
	addr := flag.String("addr", ":8443", "gRPC listen address")
	tlsCert := flag.String("tls-cert", "", "TLS cert file (required)")
	tlsKey := flag.String("tls-key", "", "TLS key file (required)")
	backend := flag.String("backend", "file", "output backend: file or syslog")
	logfile := flag.String("logfile", "./protected.log", "path to log file when backend=file")
	flag.Parse()

	if *tlsCert == "" || *tlsKey == "" {
		log.Fatal("tls-cert and tls-key are required")
	}

	creds, err := credentials.NewServerTLSFromFile(*tlsCert, *tlsKey)
	if err != nil {
		log.Fatalf("failed to load TLS credentials: %v", err)
	}

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(grpc.Creds(creds))
	svc, err := server.NewService(server.Config{
		Backend: *backend,
		LogFile: *logfile,
	})
	if err != nil {
		log.Fatalf("failed to init service: %v", err)
	}

	defer svc.Close()

	proto.RegisterLoggerServer(s, svc)

	fmt.Printf("listening on %s\n", *addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("gRPC serve failed: %v", err)
	}
}
