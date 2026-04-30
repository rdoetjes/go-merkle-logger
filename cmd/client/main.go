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

func parseClientFlags() (string, string) {
	addr := flag.String("addr", "localhost:8443", "server address")
	cafile := flag.String("ca", "", "CA cert for server TLS")
	flag.Parse()
	return *addr, *cafile
}

func buildDialOptions(cafile string) ([]grpc.DialOption, error) {
	var opts []grpc.DialOption
	// Use JSON codec registered in proto package by setting call content subtype
	// default protobuf/binary codec is used by the generated protobuf types
	if cafile != "" {
		b, err := ioutil.ReadFile(cafile)
		if err != nil {
			return nil, err
		}
		cpool := x509.NewCertPool()
		if !cpool.AppendCertsFromPEM(b) {
			return nil, fmt.Errorf("failed to append CA certs")
		}
		creds := credentials.NewClientTLSFromCert(cpool, "")
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	}
	return opts, nil
}

func main() {
	addr, cafile := parseClientFlags()
	opts, err := buildDialOptions(cafile)
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
