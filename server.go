package waddell

import (
	"fmt"
	"net"
	"sync"

	"github.com/getlantern/framed"
	"github.com/oxtoacart/bpool"
)

const (
	DEFAULT_NUM_BUFFERS = 10000
)

// Server is a waddell server
type Server struct {
	// NumBuffers: number of buffers to cache for reading and writing (balances
	// overall memory consumption against CPU usage).  Defaults to 10,000.
	NumBuffers int

	// BufferBytes: size of each buffer (this places a cap on the maxmimum
	// message size that can be transmitted).  Defaults to 65,535.
	BufferBytes int

	peers      map[PeerId]*peer // connected peers by id
	peersMutex sync.RWMutex     // protects access to peers map
	buffers    *bpool.BytePool  // pool of buffers for reading/writing
}

type peer struct {
	server *Server
	id     PeerId
	conn   net.Conn
	reader *framed.Reader
	writer *framed.Writer
}

// ListenAndServe starts the waddell server listening at the given address
func (server *Server) Serve(listener net.Listener) error {
	// Set default values
	if server.NumBuffers == 0 {
		server.NumBuffers = DEFAULT_NUM_BUFFERS
	}
	if server.BufferBytes == 0 {
		server.BufferBytes = framed.MAX_FRAME_SIZE
	}

	server.buffers = bpool.NewBytePool(server.NumBuffers, server.BufferBytes)
	server.peers = make(map[PeerId]*peer)

	for {
		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("Error accepting connection: %s", err)
		}
		c := server.addPeer(&peer{
			server: server,
			id:     randomPeerId(),
			conn:   conn,
			reader: framed.NewReader(conn),
			writer: framed.NewWriter(conn),
		})
		go c.run()
	}
}

func (server *Server) addPeer(c *peer) *peer {
	server.peersMutex.Lock()
	defer server.peersMutex.Unlock()
	server.peers[c.id] = c
	return c
}

func (server *Server) getPeer(id PeerId) *peer {
	server.peersMutex.RLock()
	defer server.peersMutex.RUnlock()
	return server.peers[id]
}

func (server *Server) removePeer(id PeerId) {
	server.peersMutex.Lock()
	defer server.peersMutex.Unlock()
	delete(server.peers, id)
}

func (peer *peer) run() {
	defer peer.conn.Close()
	defer peer.server.removePeer(peer.id)

	// Tell the peer its id
	msg := peer.server.buffers.Get()[:PEER_ID_LENGTH]
	peer.id.write(msg)
	_, err := peer.writer.Write(msg)
	if err != nil {
		log.Debugf("Unable to send peerid on connect: %s", err)
		return
	}

	// Read messages until there are no more to read
	for {
		if !peer.readNext() {
			return
		}
	}
}

func (peer *peer) readNext() bool {
	b := peer.server.buffers.Get()
	defer peer.server.buffers.Put(b)
	n, err := peer.reader.Read(b)
	if err != nil {
		return false
	}
	msg := b[:n]
	to, err := readPeerId(msg)
	if err != nil {
		// Problem determining recipient
		log.Errorf("Unable to determine recipient: ", err.Error())
		return true
	}
	cto := peer.server.getPeer(to)
	if cto == nil {
		// Recipient not found
		return true
	}
	// Set sender's id as the id in the message
	err = peer.id.write(msg)
	if err != nil {
		return true
	}
	_, err = cto.writer.Write(msg)
	if err != nil {
		cto.conn.Close()
		return true
	}
	return true
}
