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

// Message is a message read from a waddell server
type Message struct {
	// Peer is the id of the peer from/to whom this message was/will be sent
	Peer PeerId

	// Body is the content of the message
	Body []byte
}

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

	connInfoChs    chan chan *connInfo
	connErrCh      chan error
	topicsOut      map[TopicId]*topic
	topicsOutMutex sync.Mutex
	topicsIn       map[TopicId]chan *Message
	topicsInMutex  sync.Mutex
	closed         int32
}

// Connect starts the waddell client and establishes an initial connection to
// the waddell server.
//
// Note - whether or not auto reconnecting is enabled, this method doesn't
// return until a connection has been established or we've failed trying.
func (c *Client) Connect() error {
	c.connInfoChs = make(chan chan *connInfo)
	c.connErrCh = make(chan error)
	c.topicsOut = make(map[TopicId]*topic)
	c.topicsIn = make(map[TopicId]chan *Message)
	go c.stayConnected()
	go c.processInbound()
	info := c.getConnInfo()
	return info.err
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

func (c *Client) ID() (PeerId, error) {
	info := c.getConnInfo()
	if info.err != nil {
		return PeerId{}, info.err
	}
	return info.id, nil
}
