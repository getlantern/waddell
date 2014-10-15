// package waddell implements a low-latency signaling server that allows peers
// to exchange small messages (up to around 64kB) over TCP.  It is named after
// William B. Waddell, one of the founders of the Pony Express.
//
// Peers are identified by randomly assigned peer ids (type 4 UUIDs), which are
// used to address messages to the peers.  For the scheme to work, peers must
// have some out-of-band mechanism by which they can exchange peer ids.  Note
// that as soon as one peer contacts another via waddell, the 2nd peer will have
// the 1st peer's address and be able to reply using it.
//
// Peers can obtain new ids simply by reconnecting to waddell, and depending on
// security requirements it may be a good idea to do so periodically.
//
//
// Here is an example exchange between two peers:
//
//   peer 1 -> waddell server : connect
//
//   waddell server -> peer 1 : send newly assigned peer id
//
//   peer 2 -> waddell server : connect
//
//   waddell server -> peer 2 : send newly assigned peer id
//
//   (out of band)            : peer 1 lets peer 2 know about its id
//
//   peer 2 -> waddell server : send message to peer 1
//
//   waddell server -> peer 1 : deliver message from peer 2 (includes peer 2's id)
//
//   peer 1 -> waddell server : send message to peer 2
//
//   etc ..
//
//
// Message structure on the wire (bits):
//
//   0-15    Frame Length       - waddell uses github.com/getlantern/framed to
//                               frame messages. framed uses the first 16 bits
//                               of the message to indicate the length of the
//                               frame (Little Endian).
//
//   16-79   Sender Address     - 64-bit integer in Little Endian byte order
//
//   80-143  Recipient Address  - 64-bit integer in Little Endian byte order
//
//   144+    Message Body       - whatever data the client sent
//
package waddell

import (
	"encoding/binary"
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	"github.com/getlantern/golog"
)

const (
	PEER_ID_LENGTH   = 16
	WADDELL_OVERHEAD = 18 // bytes of overhead imposed by waddell
)

var (
	log = golog.LoggerFor("waddell.client")

	endianness = binary.LittleEndian
)

// PeerId is an identifier for a waddell peer, composed of two 64 bit integers
// representing a type 4 UUID.
type PeerId struct {
	part1 uint64
	part2 uint64
}

// PeerIdFromString constructs a PeerId from the string-encoded version of a
// uuid.UUID.
func PeerIdFromString(s string) (PeerId, error) {
	return readPeerId(uuid.Parse(s))
}

func (id PeerId) String() string {
	b := uuid.UUID(make([]byte, 16))
	id.write([]byte(b))
	return b.String()
}

func readPeerId(b []byte) (PeerId, error) {
	if len(b) < PEER_ID_LENGTH {
		return PeerId{}, fmt.Errorf("Insufficient data to read peer id, message may be truncated")
	}
	return PeerId{
		endianness.Uint64(b[0:8]),
		endianness.Uint64(b[8:]),
	}, nil
}

func randomPeerId() PeerId {
	id, err := readPeerId(uuid.NewRandom())
	if err != nil {
		panic(fmt.Sprintf("Unable to generate random peer id: %s", err))
	}
	return id
}

func (id PeerId) write(b []byte) error {
	if len(b) < PEER_ID_LENGTH {
		return fmt.Errorf("Insufficient room to write peer id")
	}
	endianness.PutUint64(b, id.part1)
	endianness.PutUint64(b[8:], id.part2)
	return nil
}

func (id PeerId) toBytes() []byte {
	b := make([]byte, PEER_ID_LENGTH)
	id.write(b)
	return b
}
