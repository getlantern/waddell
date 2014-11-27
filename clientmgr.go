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

	// ServerCert: PEM-encoded certificate by which to authenticate the waddell
	// server. If provided, connection to waddell is encrypted with TLS. If not,
	// connection will be made plain-text.
	ServerCert string

	// ReconnectAttempts specifies how many consecutive times to try
	// reconnecting in the event of a connection failure. See
	// Client.ReconnectAttempts for more information.
	ReconnectAttempts int

	// OnId allows optionally registering a callback to be notified whenever a
	// PeerId is assigned to the client connected to the indicated addr (i.e. on
	// each successful connection to the waddell server at addr).
	OnId func(addr string, id PeerId)

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
		dial := func() (net.Conn, error) {
			return m.Dial(addr)
		}
		if m.ServerCert != "" {
			var err error
			dial, err = Secured(dial, m.ServerCert)
			if err != nil {
				return nil, PeerId{}, err
			}
		}
		client = &Client{
			Dial:              dial,
			ReconnectAttempts: m.ReconnectAttempts,
		}
		if m.OnId != nil {
			client.OnId = func(id PeerId) {
				m.OnId(addr, id)
			}
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

func (m *ClientMgr) Close() []error {
	errors := make([]error, 0)
	m.clientsMutex.Lock()
	defer m.clientsMutex.Unlock()
	for _, client := range m.clients {
		err := client.Close()
		if err != nil {
			errors = append(errors, err)
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}
