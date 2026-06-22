package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"phonax.com/merkle/proto"
)

func parseClientFlags() (string, string, string, string) {
	addr := flag.String("addr", "localhost:8443", "server address")
	cafile := flag.String("ca", "", "CA cert for server TLS")
	tlsCert := flag.String("tls-cert", "", "client TLS cert file")
	tlsKey := flag.String("tls-key", "", "client TLS key file")
	flag.Parse()
	return *addr, *cafile, *tlsCert, *tlsKey
}

func buildDialOptions(cafile, tlsCert, tlsKey string) ([]grpc.DialOption, error) {
	var opts []grpc.DialOption

	tlsConfig := &tls.Config{}

	if cafile != "" {
		b, err := ioutil.ReadFile(cafile)
		if err != nil {
			return nil, err
		}
		cpool := x509.NewCertPool()
		if !cpool.AppendCertsFromPEM(b) {
			return nil, fmt.Errorf("failed to append CA certs")
		}
		tlsConfig.RootCAs = cpool
	} else {
		tlsConfig.InsecureSkipVerify = true
	}

	if tlsCert != "" && tlsKey != "" {
		cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert/key: %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	return opts, nil
}

func main() {
	addr, cafile, tlsCert, tlsKey := parseClientFlags()
	opts, err := buildDialOptions(cafile, tlsCert, tlsKey)
	if err != nil {
		log.Fatalf("failed to build dial opts: %v", err)
	}

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	c := proto.NewLoggerClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := c.Write(ctx, &proto.LogRequest{Application: "demo", Level: "info", Message: "hello world"})
	if err != nil {
		log.Fatalf("write: %v", err)
	}
	fmt.Printf("resp: %+v\n", res)
}
