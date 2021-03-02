package collect

import (
	"bytes"
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/debug"
	"github.com/segmentio/ksuid"
)

type NetworkStatus string

const (
	NetworkStatusAddressInUse         = "address-in-use"
	NetworkStatusConnectionRefused    = "connection-refused"
	NetworkStatusConnectionTimeout    = "connection-timeout"
	NetworkStatusConnected            = "connected"
	NetworkStatusErrorOther           = "error"
	NetworkStatusBindPermissionDenied = "bind-permission-denied"
)

type NetworkStatusResult struct {
	Status NetworkStatus `json:"status"`
}

func checkTCPConnection(progressChan chan<- interface{}, listenAddress string, dialAddress string, timeout time.Duration) (NetworkStatus, error) {
	lstn, err := net.Listen("tcp", listenAddress)
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			return NetworkStatusAddressInUse, nil
		}

		return NetworkStatusErrorOther, errors.Wrap(err, "failed to create listener")
	}
	defer lstn.Close()

	// The server may receive requests from other clients and the client request may be forwarded to
	// other servers. The client must continue to initiate new connections and send its request
	// token until the server responds with its token.
	requestToken := ksuid.New().Bytes()
	responseToken := ksuid.New().Bytes()

	go func() {
		for {
			conn, err := lstn.Accept()
			if err != nil {
				continue
			}

			if handleTestConnection(conn, requestToken, responseToken) {
				return
			}
		}
	}()

	stopAfter := time.Now().Add(timeout)

	for {
		if time.Now().After(stopAfter) {
			return NetworkStatusConnectionTimeout, nil
		}

		conn, err := net.DialTimeout("tcp", dialAddress, 50*time.Millisecond)
		if err != nil {
			if strings.Contains(err.Error(), "i/o timeout") {
				progressChan <- err
				time.Sleep(time.Second)
				continue
			}
			if strings.Contains(err.Error(), "connection refused") {
				return NetworkStatusConnectionRefused, nil
			}
			return NetworkStatusErrorOther, errors.Wrap(err, "failed to dial")
		}

		if verifyConnectionToServer(conn, requestToken, responseToken) {
			return NetworkStatusConnected, nil
		}

		progressChan <- errors.New("failed to verify connection to server")
		time.Sleep(time.Second)
	}
}

func handleTestConnection(conn net.Conn, requestToken []byte, responseToken []byte) bool {
	defer func() {
		if err := conn.Close(); err != nil {
			debug.Println(err.Error())
		}
	}()

	if err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		debug.Printf("Server failed to set read deadline: %v\n", err)
		return false
	}

	buf := make([]byte, 1024)
	_, err := conn.Read(buf)
	if err != nil {
		debug.Printf("Server failed to read: %v\n", err)
		return false
	}

	if !bytes.Contains(buf, requestToken) {
		return false
	}

	if err := conn.SetWriteDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		debug.Printf("Server failed to set write deadline: %v\n", err)
		return false
	}

	if _, err := conn.Write(responseToken); err != nil {
		debug.Printf("Server failed to write: %v\n", err)
		return false
	}

	return true
}

func verifyConnectionToServer(conn net.Conn, requestToken []byte, responseToken []byte) bool {
	defer func() {
		if err := conn.Close(); err != nil {
			debug.Println(err.Error())
		}
	}()

	if err := conn.SetWriteDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		debug.Printf("Client failed to set write deadline: %v\n", err)
		return false
	}

	_, err := conn.Write(requestToken)
	if err != nil {
		return false
	}

	if err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		debug.Printf("Client failed to set read deadline: %v\n", err)
		return false
	}

	buf := make([]byte, 1024)
	_, err = conn.Read(buf)
	if err != nil {
		return false
	}

	if !bytes.Contains(buf, responseToken) {
		return false
	}

	return true
}
