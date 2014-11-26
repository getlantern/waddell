package waddell

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/getlantern/framed"
	"github.com/getlantern/keyman"
)

var (
	maxReconnectDelay = 5 * time.Second
	reconnectBackoff  = 100 * time.Millisecond
)

// Message is a message read from a waddell server
type Message struct {
	// From is the id of the peer who sent the message
	From PeerId

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

	connInfoChs chan chan *connInfo
	connErrCh   chan error
	closed      int32
}

// Connect starts the waddell client and establishes an initial connection to
// the waddell server.
//
// Note - whether or not auto reconnecting is enabled, this method doesn't
// return until a connection has been established or we've failed trying.
func (c *Client) Connect() error {
	c.connInfoChs = make(chan chan *connInfo)
	c.connErrCh = make(chan error)
	go c.run()
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

// Receive reads the next Message from waddell.
func (c *Client) Receive() (*Message, error) {
	info := c.getConnInfo()
	if info.err != nil {
		return nil, info.err
	}
	msg, err := info.receive()
	if err != nil {
		c.connError(err)
	}
	return msg, err
}

func (info *connInfo) receive() (*Message, error) {
	log.Trace("Receiving")
	frame, err := info.reader.ReadFrame()
	log.Tracef("Received %d: %s", len(frame), err)
	if err != nil {
		return nil, err
	}
	peer, err := readPeerId(frame)
	if err != nil {
		return nil, err
	}
	return &Message{
		From: peer,
		Body: frame[PeerIdLength:],
	}, nil
}

// Send sends the given body to the indiciated peer via waddell.
func (c *Client) Send(to PeerId, body []byte) error {
	return c.SendPieces(to, body)
}

// SendPieces sends the given multi-piece body to the indiciated peer via
// waddell.
func (c *Client) SendPieces(to PeerId, bodyPieces ...[]byte) error {
	info := c.getConnInfo()
	if info.err != nil {
		return info.err
	}
	pieces := append([][]byte{to.toBytes()}, bodyPieces...)
	_, err := info.writer.WritePieces(pieces...)
	if err != nil {
		c.connError(err)
	}
	return err
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

func (c *Client) Close() error {
	justClosed := atomic.CompareAndSwapInt32(&c.closed, 0, 1)
	if justClosed {
		info := c.getConnInfo()
		if info.conn != nil {
			return info.conn.Close()
		}
		close(c.connInfoChs)
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

type connInfo struct {
	id     PeerId
	conn   net.Conn
	reader *framed.Reader
	writer *framed.Writer
	err    error
}

func (c *Client) run() {
	var info *connInfo
	for {
		select {
		case err := <-c.connErrCh:
			log.Tracef("Encountered error, disconnecting: %s", err)
			if info != nil {
				info.conn.Close()
				info = nil
			}
		case infoCh, open := <-c.connInfoChs:
			if !open {
				log.Trace("connInfoChs closed, done processing")
				return
			}
			if info != nil {
				infoCh <- info
				continue
			}
			info = c.connect()
			infoCh <- info
		}
	}
}

func (c *Client) connect() *connInfo {
	log.Trace("Connecting ...")
	var lastErr error
	consecutiveFailures := 0
	for {
		if consecutiveFailures > c.ReconnectAttempts {
			log.Tracef("Done trying to connect: %s", lastErr)
			return &connInfo{
				err: fmt.Errorf("Unable to connect: %s", lastErr),
			}
		}
		info, err := c.connectOnce()
		if err == nil {
			return info
		}
		log.Tracef("Unable to connect: %s", err)
		lastErr = err
		info = nil
		consecutiveFailures += 1
	}
}

func (c *Client) connectOnce() (*connInfo, error) {
	conn, err := c.Dial()
	if err != nil {
		return nil, err
	}
	info := &connInfo{
		conn:   conn,
		reader: framed.NewReader(conn),
		writer: framed.NewWriter(conn),
	}
	// Read first message to get our PeerId
	msg, err := info.receive()
	if err != nil {
		return nil, fmt.Errorf("Unable to get peerid: %s", err)
	}
	info.id = msg.From
	return info, nil
}

func (c *Client) connError(err error) {
	c.connErrCh <- err
}

func (c *Client) getConnInfo() *connInfo {
	infoCh := make(chan *connInfo)
	c.connInfoChs <- infoCh
	return <-infoCh
}
