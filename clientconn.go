package waddell

import (
	"fmt"
	"net"

	"github.com/getlantern/framed"
)

type Message struct {
	Peer PeerId
	Body []byte
}

type Client struct {
	id     PeerId
	conn   net.Conn
	reader *framed.Reader
	writer *framed.Writer
}

func Connect(serverAddr string) (*Client, error) {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return nil, fmt.Errorf("Unable to dial server: %s", err)
	}
	c := &Client{
		conn:   conn,
		reader: framed.NewReader(conn),
		writer: framed.NewWriter(conn),
	}
	// Read first message to get our PeerId
	msg, err := c.Read(make([]byte, PEER_ID_LENGTH))
	if err != nil {
		return nil, fmt.Errorf("Unable to get peerid: %s", err)
	}
	c.id = msg.Peer
	return c, nil
}

func (c *Client) Read(b []byte) (*Message, error) {
	n, err := c.reader.Read(b)
	if err != nil {
		return nil, err
	}
	peer, err := readPeerId(b)
	if err != nil {
		return nil, err
	}
	return &Message{
		Peer: peer,
		Body: b[PEER_ID_LENGTH:n],
	}, nil
}

func (c *Client) Write(to PeerId, body []byte) error {
	_, err := c.writer.WritePieces(to.toBytes(), body)
	return err
}
