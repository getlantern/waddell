package waddell

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getlantern/testify/assert"
)

const (
	Hello         = "Hello"
	HelloYourself = "Hello %s!"

	NumPeers = 100

	TestTopic = TopicId(1)
)

// TestPeerIdRoundTrip makes sure that we can write and read a PeerId to/from a
// byte array.
func TestPeerIdRoundTrip(t *testing.T) {
	b := make([]byte, PeerIdLength)
	orig := randomPeerId()
	orig.write(b)
	read, err := readPeerId(b)
	if err != nil {
		t.Errorf("Unable to read peer id: %s", err)
	} else {
		assert.Equal(t, orig, read)
	}
}

func TestPeerIdStringRoundTrip(t *testing.T) {
	orig := randomPeerId()
	read, err := PeerIdFromString(orig.String())
	if err != nil {
		t.Errorf("Unable to read peer id: %s", err)
	} else {
		assert.Equal(t, orig, read)
	}
}

func TestTopicIdRoundTrip(t *testing.T) {
	orig := TopicId(5)
	read, err := readTopicId(orig.toBytes())
	if err != nil {
		t.Errorf("Error reading topic id: %s", err)
	} else {
		assert.Equal(t, orig, read)
	}
}

func TestTopicIdTruncation(t *testing.T) {
	orig := TopicId(5)
	_, err := readTopicId(orig.toBytes()[:1])
	assert.Error(t, err)
}

func TestBadDialerWithNoReconnect(t *testing.T) {
	client := &Client{
		ReconnectAttempts: 0,
		Dial: func() (net.Conn, error) {
			return nil, fmt.Errorf("I won't dial, no way!")
		},
	}
	defer client.Close()
	_, err := client.Connect()
	assert.Error(t, err, "Connecting with no reconnect attempts should have failed")
}

func TestBadDialerWithMultipleReconnect(t *testing.T) {
	client := &Client{
		ReconnectAttempts: 2,
		Dial: func() (net.Conn, error) {
			return nil, fmt.Errorf("I won't dial, no way!")
		},
	}
	defer client.Close()
	start := time.Now()
	_, err := client.Connect()
	delta := time.Now().Sub(start)
	assert.Error(t, err, "Connecting with no reconnect attempts should have failed")
	expectedDelta := reconnectDelayInterval * 3
	assert.True(t, delta >= expectedDelta, fmt.Sprintf("Reconnecting didn't wait long enough. Should have waited %s, only waited %s", expectedDelta, delta))
}

func TestPeersPlainText(t *testing.T) {
	doTestPeers(t, false)
}

func TestPeersTLS(t *testing.T) {
	doTestPeers(t, true)
}

type clientWithId struct {
	id     PeerId
	client *Client
}

func doTestPeers(t *testing.T, useTLS bool) {
	pkfile := ""
	certfile := ""
	if useTLS {
		pkfile = "waddell_test_pk.pem"
		certfile = "waddell_test_cert.pem"
	}

	listener, err := Listen("localhost:0", pkfile, certfile)
	if err != nil {
		log.Fatalf("Unable to listen: %s", err)
	}

	go func() {
		server := &Server{}
		err = server.Serve(listener)
		if err != nil {
			log.Fatalf("Unable to start server: %s", err)
		}
	}()

	serverAddr := listener.Addr().String()

	succeedOnDial := int32(0)
	dial := func() (net.Conn, error) {
		if atomic.CompareAndSwapInt32(&succeedOnDial, 0, 1) {
			return nil, fmt.Errorf("Deliberately failing the first time around")
		}
		return net.Dial("tcp", serverAddr)
	}
	if useTLS {
		certBytes, err := ioutil.ReadFile(certfile)
		if err != nil {
			log.Fatalf("Unable to read cert from file: %s", err)
		}
		cert := string(certBytes)
		dial, err = Secured(dial, cert)
		if err != nil {
			log.Fatalf("Unable to secure dial function: %s", err)
		}
	}
	connect := func() *clientWithId {
		client := &Client{
			Dial:              dial,
			ReconnectAttempts: 1,
		}
		id, err := client.Connect()
		if err != nil {
			log.Fatalf("Unable to connect client: %s", err)
		}
		return &clientWithId{id, client}
	}

	peersCh := make(chan *clientWithId, NumPeers)
	// Connect clients
	for i := 0; i < NumPeers; i++ {
		go func() {
			peer := connect()
			assert.NoError(t, err, "Unable to connect peer")
			peersCh <- peer
		}()
	}

	peers := make([]*clientWithId, 0, NumPeers)
	for i := 0; i < NumPeers; i++ {
		peers = append(peers, <-peersCh)
	}
	defer func() {
		for _, peer := range peers {
			peer.client.Close()
		}
	}()

	var wg sync.WaitGroup
	wg.Add(NumPeers)

	// Send some large data to a peer that doesn't read, just to make sure we
	// handle blocked readers okay
	badPeer := connect()
	ld := largeData()
	for i := 0; i < 10; i++ {
		if err != nil {
			log.Fatalf("Unable to get peer id: %s", err)
		}
		badPeer.client.Out(TestTopic) <- &Message{badPeer.id, ld}
	}

	// Simulate readers and writers
	for i := 0; i < NumPeers; i++ {
		peer := peers[i]
		isWriter := i%2 == 1

		if isWriter {
			go func() {
				// Simulate writer
				defer wg.Done()

				// Write to each reader
				for j := 0; j < NumPeers; j += 2 {
					recip := peers[j]
					peer.client.Out(TestTopic) <- &Message{recip.id, []byte(Hello)}
					if err != nil {
						log.Fatalf("Unable to write hello: %s", err)
					} else {
						resp := <-peer.client.In(TestTopic)
						assert.Equal(t, fmt.Sprintf(HelloYourself, peer.id), string(resp.Body), "Response should match expected.")
						assert.Equal(t, recip.id, resp.Peer, "Peer on response should match expected")
					}
				}
			}()
		} else {
			go func() {
				// Simulate reader
				defer wg.Done()

				// Read from all readers
				for j := 1; j < NumPeers; j += 2 {
					err := peer.client.SendKeepAlive()
					if err != nil {
						log.Fatalf("Unable to send KeepAlive: %s", err)
					}
					msg := <-peer.client.In(TestTopic)
					assert.Equal(t, Hello, string(msg.Body), "Hello message should match expected")
					peer.client.Out(TestTopic) <- &Message{msg.Peer, []byte(fmt.Sprintf(HelloYourself, msg.Peer))}
					if err != nil {
						log.Fatalf("Unable to write response to HELLO message: %s", err)
					}
				}
			}()
		}
	}

	wg.Wait()
}

func largeData() []byte {
	b := make([]byte, 60000)
	for i := 0; i < len(b); i++ {
		b[i] = byte(rand.Int())
	}
	return b
}
