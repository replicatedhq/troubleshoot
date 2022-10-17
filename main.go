// TODO: Remove me before merging PR. This is for testing purposes only
// A very simple application for making a TLS connection to a redis server
package main

// TODO: Remove me before merging PR. This is for testing purposes only

// Connect to redis after starting it using
// docker run -it --rm -v $(pwd)/tls:/redis/tls -p6379:6379 -p6380:6380 redis redis-server --requirepass replicated --tls-port 6380 --port 6379 --tls-cert-file /redis/tls/server.pem --tls-key-file /redis/tls/server-key.pem --tls-ca-cert-file /redis/tls/ca.pem --loglevel debug

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	"github.com/go-redis/redis/v7"
)

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

func connect() *redis.Client {
	fmt.Println("Making plain text connection to redis now")
	return redis.NewClient(&redis.Options{
		Addr:	  "localhost:6379",
		Password: "replicated",
		DB:		  0,  // use default DB
	})
}

func connectTLS(clientCert, clientKey, caPEM []byte, serverName string) *redis.Client {
    rootCA := x509.NewCertPool()
	if ok := rootCA.AppendCertsFromPEM(caPEM); !ok {
       panic("failed to append CA to root CAs")
    }

	certPair, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		log.Fatal(err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{certPair},
		RootCAs: rootCA,
		ServerName: serverName,
	}

	fmt.Println("Testing connection")
    tcpConn, err := tls.Dial("tcp", "localhost:6380", tlsCfg)
    if err != nil {
        panic(fmt.Sprintf("Failed to connect: %s", err))
    }
    defer tcpConn.Close()

	fmt.Println("Making TLS connection to redis now")
	return redis.NewClient(&redis.Options{
		TLSConfig: tlsCfg,
		Addr:	  "localhost:6380",
		Password: "replicated",
		DB:		  0,  // use default DB
	})
}

func main() {
	// This will have come from some path config somewhere
	certFile := "./tls/client.pem"
	keyFile := "./tls/client-key.pem"
	caFile := "./tls/ca.pem"
	serverName := "server"

	certPEMBlock, err := os.ReadFile(certFile)
	checkErr(err)

	keyPEMBlock, err := os.ReadFile(keyFile)
	checkErr(err)

	caPEM, err := os.ReadFile(caFile)
	checkErr(err)

	rdbTLS := connectTLS(certPEMBlock, keyPEMBlock, caPEM, serverName)
	defer rdbTLS.Close()

	err = rdbTLS.Set("key", "My TLS Value for Replicated", 0).Err()
    if err != nil {
        panic(err)
    }

    val, err := rdbTLS.Get("key").Result()
    if err != nil {
        panic(err)
    }
    fmt.Println("key:", val)

	rdb := connect()
	defer rdb.Close()

	err = rdb.Set("key", "My Value for Replicated", 0).Err()
    if err != nil {
        panic(err)
    }

    val, err = rdb.Get("key").Result()
    if err != nil {
        panic(err)
    }
    fmt.Println("key:", val)
}
