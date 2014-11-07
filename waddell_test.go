package waddell

import (
	"io/ioutil"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/getlantern/testify/assert"
)

const (
	Hello         = "Hello"
	HelloYourself = "Hello Yourself!"
)

// TestPeerIdRoundTrip makes sure that we can write and read a PeerId to/from a
// byte array.
func TestPeerIdRoundTrip(t *testing.T) {
	b := make([]byte, PEER_ID_LENGTH)
	orig := randomPeerId()
	orig.write(b)
	read, err := readPeerId(b)
	if err != nil {
		t.Errorf("Unable to read peer id: %s", err)
	} else {
		if read != orig {
			t.Errorf("Read did not match original.  Expected: %s, Got: %s", orig, read)
		}
	}
}

func TestPeersPlainText(t *testing.T) {
	doTestPeers(t, false)
}

func TestPeersTLS(t *testing.T) {
	doTestPeers(t, true)
}

func doTestPeers(t *testing.T, useTLS bool) {
	pkfile := ""
	certfile := ""
	cert := ""

	if useTLS {
		pkfile = "waddell_test_pk.pem"
		certfile = "waddell_test_cert.pem"
		certBytes, err := ioutil.ReadFile(certfile)
		if err != nil {
			t.Fatalf("Unable to read cert from file: %s", err)
		}
		cert = string(certBytes)
	}

	listener, err := Listen("localhost:0", pkfile, certfile)
	if err != nil {
		t.Fatalf("Unable to listen: %s", err)
	}

	go func() {
		server := &Server{}
		err = server.Serve(listener)
		if err != nil {
			t.Fatalf("Unable to start server: %s", err)
		}
	}()

	serverAddr := listener.Addr().String()

	conn1, peer1, err := Dial(serverAddr, cert)
	if err != nil {
		t.Fatalf("Unable to connect peer1: %s", err)
	}
	defer conn1.Close()

	conn2, peer2, err := Dial(serverAddr, cert)
	if err != nil {
		t.Fatalf("Unable to connect peer1: %s", err)
	}
	defer conn2.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		// Simulate peer 2
		defer wg.Done()
		readBuffer := make([]byte, 100)

		err := peer2.Send(peer1.id, []byte(Hello))
		if err != nil {
			t.Fatalf("Unable to write hello: %s", err)
		} else {
			resp, err := peer2.Receive(readBuffer)
			if err != nil {
				t.Fatalf("Unable to read response to hello: %s", err)
			} else {
				assert.Equal(t, HelloYourself, string(resp.Body), "Response should match expected.")
				assert.Equal(t, peer1.ID(), resp.From, "Peer on response should match expected")
			}
		}
	}()

	go func() {
		// Simulate peer 1
		defer wg.Done()
		readBuffer := make([]byte, 100)

		err := peer1.SendKeepAlive()
		if err != nil {
			t.Fatalf("Unable to send KeepAlive: %s", err)
		}
		msg, err := peer1.Receive(readBuffer)
		if err != nil {
			t.Fatalf("Unable to read hello message: %s", err)
		}
		assert.Equal(t, Hello, string(msg.Body), "Hello message should match expected")
		assert.Equal(t, peer2.ID(), msg.From, "Peer on hello message should match expected")
		err = peer1.Send(peer2.id, []byte(HelloYourself))
		if err != nil {
			t.Fatalf("Unable to write response to HELLO message: %s", err)
		}
	}()

	wg.Wait()
}

// waitForServer waits for a TCP server to start at the given address, waiting
// up to the given limit and reporting an error to the given testing.T if the
// server didn't start within the time limit.
func waitForServer(addr string, limit time.Duration, t *testing.T) {
	cutoff := time.Now().Add(limit)
	for {
		if time.Now().After(cutoff) {
			t.Errorf("Server never came up at address %s", addr)
			return
		}
		c, err := net.DialTimeout("tcp", addr, limit)
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}
