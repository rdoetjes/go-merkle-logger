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

func main() {
	addr := flag.String("addr", "localhost:8443", "server address")
	cafile := flag.String("ca", "", "CA cert for server TLS")
	flag.Parse()

	var opts []grpc.DialOption
	// Use JSON codec registered in proto package by setting call content subtype
	opts = append(opts, grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
	if *cafile != "" {
		b, err := ioutil.ReadFile(*cafile)
		if err != nil {
			log.Fatalf("read ca: %v", err)
		}
		cpool := x509.NewCertPool()
		cpool.AppendCertsFromPEM(b)
		creds := credentials.NewClientTLSFromCert(cpool, "")
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})))
	}

	conn, err := grpc.Dial(*addr, opts...)
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
