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
	msg, err := c.Receive(make([]byte, PEER_ID_LENGTH))
	if err != nil {
		return nil, fmt.Errorf("Unable to get peerid: %s", err)
	}
	c.id = msg.From
	return c, nil
}

// Dial opens a Waddell client to the given addr. If cert is specified, the
// connection will use TLS and authenticate the server using the given cert.
// Cert is assumed to be PEM encoded.
func Dial(addr string, cert string) (net.Conn, *Client, error) {
	var conn net.Conn
	var err error
	if cert == "" {
		conn, err = net.Dial("tcp", addr)
	} else {
		conn, err = dialTLS(addr, cert)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to dial server at addr %s: %s", addr, err)
	}
	client, err := Connect(conn)
	return conn, client, err
}

func dialTLS(addr string, cert string) (net.Conn, error) {
	c, err := keyman.LoadCertificateFromPEMBytes([]byte(cert))
	if err != nil {
		return nil, err
	}
	return tls.Dial("tcp", addr, &tls.Config{
		RootCAs:    c.PoolContainingCert(),
		ServerName: c.X509().Subject.CommonName,
	})
}

func (c *Client) ID() PeerId {
	return c.id
}

// Receive reads the next Message from waddell, using the given buffer to
// receive the message.
func (c *Client) Receive(b []byte) (*Message, error) {
	log.Trace("Receiving")
	n, err := c.reader.Read(b)
	log.Tracef("Received %d: %s", n, err)
	if err != nil {
		return nil, err
	}
	peer, err := readPeerId(b)
	if err != nil {
		return nil, err
	}
	return &Message{
		From: peer,
		Body: b[PEER_ID_LENGTH:n],
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
