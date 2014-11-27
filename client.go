package waddell

import (
	"crypto/tls"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getlantern/keyman"
)

var (
	maxReconnectDelay      = 5 * time.Second
	reconnectDelayInterval = 100 * time.Millisecond
)

// Client is a client of a waddell server
type Client struct {
	// Dial is a function that dials the waddell server
	Dial func() (net.Conn, error)

	// ReconnectAttempts specifies how many consecutive times to try
	// reconnecting in the event of a connection failure.
	//
	// Note - when auto reconnecting is enabled, the client will never resend
	// messages, it will simply reopen the connection.
	ReconnectAttempts int

	// IdChannel is a channel that publishes this client's PeerId as it first
	// becomes available and changes on subsequent reconnects. A channel is used
	// here because the PeerId changes with each reconnect.
	IdChannel <-chan PeerId
	idChannel chan PeerId

	connInfoChs    chan chan *connInfo
	connErrCh      chan error
	topicsOut      map[TopicId]*topic
	topicsOutMutex sync.Mutex
	topicsIn       map[TopicId]chan *MessageIn
	topicsInMutex  sync.Mutex
	closed         int32
}

// Connect starts the waddell client and establishes an initial connection to
// the waddell server, returning the initial PeerId.
//
// Note - if the client automatically reconnects, its peer ID will change. You
// can obtain the new id from IdChannel.
//
// Note - whether or not auto reconnecting is enabled, this method doesn't
// return until a connection has been established or we've failed trying.
func (c *Client) Connect() (PeerId, error) {
	c.idChannel = make(chan PeerId, 100)
	c.IdChannel = c.idChannel
	c.connInfoChs = make(chan chan *connInfo)
	c.connErrCh = make(chan error)
	c.topicsOut = make(map[TopicId]*topic)
	c.topicsIn = make(map[TopicId]chan *MessageIn)
	go c.stayConnected()
	go c.processInbound()
	info := c.getConnInfo()
	return info.id, info.err
}

// Secured wraps the given dial function with TLS support, authenticating the
// waddell server using the supplied cert (assumed to be PEM encoded).
func Secured(dial func() (net.Conn, error), cert string) (func() (net.Conn, error), error) {
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

// SendKeepAlive sends a keep alive message to the server to keep the underlying
// connection open.
func (c *Client) SendKeepAlive() error {
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
	justClosed := atomic.CompareAndSwapInt32(&c.closed, 0, 1)
	if justClosed {
		info := c.getConnInfo()
		if info.conn != nil {
			return info.conn.Close()
		}
		close(c.connInfoChs)
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
	}
	return nil
}
