package waddell

import (
	"net"
	"sync"
)

// ClientMgr provides a mechanism for managing connections to multiple waddell
// servers.
type ClientMgr struct {
	// Dial is a function that dials the waddell server at the given addr.
	Dial func(addr string) (net.Conn, error)

	// ReconnectAttempts specifies how many consecutive times to try
	// reconnecting in the event of a connection failure. See
	// Client.ReconnectAttempts for more information.
	ReconnectAttempts int

	clients      map[string]*Client
	ids          map[string]PeerId
	clientsMutex sync.Mutex
}

func (m *ClientMgr) ClientTo(addr string) (*Client, PeerId, error) {
	m.clientsMutex.Lock()
	defer m.clientsMutex.Unlock()
	if m.clients == nil {
		m.clients = make(map[string]*Client)
		m.ids = make(map[string]PeerId)
	}
	client := m.clients[addr]
	if client == nil {
		client = &Client{
			Dial: func() (net.Conn, error) {
				return m.Dial(addr)
			},
			ReconnectAttempts: m.ReconnectAttempts,
		}
		id, err := client.Connect()
		if err != nil {
			return nil, id, err
		}
		m.clients[addr] = client
		m.ids[addr] = id
	}
	id := m.ids[addr]
	return client, id, nil
}
