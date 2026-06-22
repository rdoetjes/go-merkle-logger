package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"phonax.com/merkle/internal/server"
	"phonax.com/merkle/proto"
)

type cliConfig struct {
	addr    string
	tlsCert string
	tlsKey  string
	caCert  string
	backend string
	logfile string
}

func parseFlags() cliConfig {
	addr := flag.String("addr", ":8443", "gRPC listen address")
	tlsCert := flag.String("tls-cert", "", "TLS cert file (required)")
	tlsKey := flag.String("tls-key", "", "TLS key file (required)")
	caCert := flag.String("ca", "", "CA cert file for client authentication (optional, enables mTLS)")
	backend := flag.String("backend", "file", "output backend: file or syslog")
	logfile := flag.String("logfile", "./protected.log", "path to log file when backend=file")
	flag.Parse()
	return cliConfig{addr: *addr, tlsCert: *tlsCert, tlsKey: *tlsKey, caCert: *caCert, backend: *backend, logfile: *logfile}
}

func loadTLSCredentials(cfg cliConfig) (credentials.TransportCredentials, error) {
	if cfg.caCert == "" {
		return credentials.NewServerTLSFromFile(cfg.tlsCert, cfg.tlsKey)
	}

	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair(cfg.tlsCert, cfg.tlsKey)
	if err != nil {
		return nil, err
	}

	// Load CA certificate
	caPem, err := os.ReadFile(cfg.caCert)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caPem) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig), nil
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

	creds, err := loadTLSCredentials(cfg)
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
