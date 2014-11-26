package waddell

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/getlantern/framed"
	"github.com/getlantern/keyman"
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
	id     PeerId
	conn   net.Conn
	reader *framed.Reader
	writer *framed.Writer
}

// Connect connects to a waddell server
func Connect(conn net.Conn) (*Client, error) {
	c := &Client{
		conn:   conn,
		reader: framed.NewReader(conn),
		writer: framed.NewWriter(conn),
	}
	// Read first message to get our PeerId
	msg, err := c.Receive()
	if err != nil {
		return nil, fmt.Errorf("Unable to get peerid: %s", err)
	}
	c.id = msg.From
	return c, nil
}

// ConnectTLS is like connect, but it first wraps the given conn with TLS using
// the supplied cert to authenticate the server (assumed to be PEM encoded).
func ConnectTLS(conn net.Conn, cert string) (*Client, error) {
	c, err := keyman.LoadCertificateFromPEMBytes([]byte(cert))
	if err != nil {
		return nil, err
	}
	return Connect(tls.Client(conn, &tls.Config{
		RootCAs:    c.PoolContainingCert(),
		ServerName: c.X509().Subject.CommonName,
	}))
}

func (c *Client) ID() PeerId {
	return c.id
}

// Receive reads the next Message from waddell.
func (c *Client) Receive() (*Message, error) {
	log.Trace("Receiving")
	frame, err := c.reader.ReadFrame()
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
	pieces := append([][]byte{to.toBytes()}, bodyPieces...)
	_, err := c.writer.WritePieces(pieces...)
	return err
}

// SendKeepAlive sends a keep alive message to the server to keep the underlying
// connection open.
func (c *Client) SendKeepAlive() error {
	_, err := c.writer.Write(keepAlive)
	return err
}
