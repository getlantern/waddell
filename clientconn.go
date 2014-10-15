package waddell

import (
	"fmt"
	"net"

	"github.com/getlantern/framed"
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

// Send sends the given multi-piece body to the indiciated peer via waddell.
func (c *Client) SendPieces(to PeerId, bodyPieces ...[]byte) error {
	pieces := append([][]byte{to.toBytes()}, bodyPieces...)
	_, err := c.writer.WritePieces(pieces...)
	return err
}
