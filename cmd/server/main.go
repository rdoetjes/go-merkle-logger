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

type cliConfig struct {
	addr    string
	tlsCert string
	tlsKey  string
	backend string
	logfile string
}

func parseFlags() cliConfig {
	addr := flag.String("addr", ":8443", "gRPC listen address")
	tlsCert := flag.String("tls-cert", "", "TLS cert file (required)")
	tlsKey := flag.String("tls-key", "", "TLS key file (required)")
	backend := flag.String("backend", "file", "output backend: file or syslog")
	logfile := flag.String("logfile", "./protected.log", "path to log file when backend=file")
	flag.Parse()
	return cliConfig{addr: *addr, tlsCert: *tlsCert, tlsKey: *tlsKey, backend: *backend, logfile: *logfile}
}

func loadTLSCredentials(certFile, keyFile string) (credentials.TransportCredentials, error) {
	return credentials.NewServerTLSFromFile(certFile, keyFile)
}

func createListener(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

func newGRPCServer(creds credentials.TransportCredentials) *grpc.Server {
	if creds != nil {
		return grpc.NewServer(grpc.Creds(creds))
	}
	return grpc.NewServer()
}

func initService(cfg cliConfig) (*server.Service, error) {
	return server.NewService(server.Config{Backend: cfg.backend, LogFile: cfg.logfile})
}

func registerAndServe(s *grpc.Server, svc *server.Service, lis net.Listener, addr string) error {
	proto.RegisterLoggerServer(s, svc)
	fmt.Printf("listening on %s\n", addr)
	return s.Serve(lis)
}

func main() {
	cfg := parseFlags()
	if cfg.tlsCert == "" || cfg.tlsKey == "" {
		log.Fatal("tls-cert and tls-key are required")
	}

	creds, err := loadTLSCredentials(cfg.tlsCert, cfg.tlsKey)
	if err != nil {
		log.Fatalf("failed to load TLS credentials: %v", err)
	}

	lis, err := createListener(cfg.addr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := newGRPCServer(creds)

	svc, err := initService(cfg)
	if err != nil {
		log.Fatalf("failed to init service: %v", err)
	}
	defer svc.Close()

	if err := registerAndServe(s, svc, lis, cfg.addr); err != nil {
		log.Fatalf("gRPC serve failed: %v", err)
	}
}
