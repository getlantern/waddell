package waddell

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getlantern/keyman"
)

var (
	maxReconnectDelay      = 5 * time.Second
	reconnectDelayInterval = 100 * time.Millisecond

	notConnectedError = fmt.Errorf("Client not yet connected")
	closedError       = fmt.Errorf("Client closed")
)

// Client is a client of a waddell server
type Client struct {
	// Dial is a function that dials the waddell server
	Dial DialFunc

	// ServerCert: PEM-encoded certificate by which to authenticate the waddell
	// server. If provided, connection to waddell is encrypted with TLS. If not,
	// connection will be made plain-text.
	ServerCert string

	// ReconnectAttempts specifies how many consecutive times to try
	// reconnecting in the event of a connection failure.
	//
	// Note - when auto reconnecting is enabled, the client will never resend
	// messages, it will simply reopen the connection.
	ReconnectAttempts int

	// OnId allows optionally registering a callback to be notified whenever a
	// PeerId is assigned to this client (i.e. on each successful connection to
	// the waddell server).
	OnId func(id PeerId)

	connInfoChs    chan chan *connInfo
	connErrCh      chan error
	topicsOut      map[TopicId]*topic
	topicsOutMutex sync.Mutex
	topicsIn       map[TopicId]chan *MessageIn
	topicsInMutex  sync.Mutex
	connected      int32
	closed         int32
}

// DialFunc is a function for dialing a waddell server.
type DialFunc func() (net.Conn, error)

// Connect starts the waddell client and establishes an initial connection to
// the waddell server, returning the initial PeerId.
//
// Note - if the client automatically reconnects, its peer ID will change. You
// can obtain the new id from IdChannel.
//
// Note - whether or not auto reconnecting is enabled, this method doesn't
// return until a connection has been established or we've failed trying.
func (c *Client) Connect() (PeerId, error) {
	alreadyConnected := !atomic.CompareAndSwapInt32(&c.connected, 0, 1)
	if alreadyConnected {
		return PeerId{}, fmt.Errorf("Client already connected")
	}

	var err error
	if c.ServerCert != "" {
		c.Dial, err = secured(c.Dial, c.ServerCert)
		if err != nil {
			return PeerId{}, err
		}
	}

	c.connInfoChs = make(chan chan *connInfo)
	c.connErrCh = make(chan error)
	c.topicsOut = make(map[TopicId]*topic)
	c.topicsIn = make(map[TopicId]chan *MessageIn)
	go c.stayConnected()
	go c.processInbound()
	info := c.getConnInfo()
	return info.id, info.err
}

// SendKeepAlive sends a keep alive message to the server to keep the underlying
// connection open.
func (c *Client) SendKeepAlive() error {
	if !c.hasConnected() {
		return notConnectedError
	}
	if c.isClosed() {
		return closedError
	}

	info := c.getConnInfo()
	if info.err != nil {
		return info.err
	}
	_, err := info.writer.Write(keepAlive)
	if err != nil {
		c.connError(err)
	}
	return err
}

// Close closes this client and associated resources
func (c *Client) Close() error {
	if !c.hasConnected() {
		return notConnectedError
	}
	justClosed := atomic.CompareAndSwapInt32(&c.closed, 0, 1)
	if !justClosed {
		return closedError
	}

	var err error
	log.Trace("Closing client")
	c.topicsInMutex.Lock()
	defer c.topicsInMutex.Unlock()
	c.topicsOutMutex.Lock()
	defer c.topicsOutMutex.Unlock()
	for _, t := range c.topicsOut {
		close(t.out)
	}
	for _, ch := range c.topicsIn {
		close(ch)
	}
	info := c.getConnInfo()
	if info.conn != nil {
		err = info.conn.Close()
		log.Trace("Closed client connection")
	}
	close(c.connInfoChs)
	return err
}

// secured wraps the given dial function with TLS support, authenticating the
// waddell server using the supplied cert (assumed to be PEM encoded).
func secured(dial DialFunc, cert string) (DialFunc, error) {
	c, err := keyman.LoadCertificateFromPEMBytes([]byte(cert))
	if err != nil {
		return nil, err
	}
	tlsConfig := &tls.Config{
		RootCAs:    c.PoolContainingCert(),
		ServerName: c.X509().Subject.CommonName,
	}
	return func() (net.Conn, error) {
		conn, err := dial()
		if err != nil {
			return nil, err
		}
		return tls.Client(conn, tlsConfig), nil
	}, nil
}

func (c *Client) hasConnected() bool {
	return c.connected == 1
}

func (c *Client) isClosed() bool {
	return c.closed == 1
}
