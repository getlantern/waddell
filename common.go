package waddell

import (
	"encoding/binary"
	"fmt"

	"code.google.com/p/go-uuid/uuid"
)

const (
	PEER_ID_LENGTH = 16
)

// peerId is an identifier for a waddell peer, composed of two 64 bit
// integers representing a type 4 UUID
type PeerId struct {
	part1 int64
	part2 int64
}

func PeerIdFromString(s string) (PeerId, error) {
	return readPeerId(uuid.Parse(s))
}

func readPeerId(b []byte) (PeerId, error) {
	part1, n := binary.Varint(b[0:8])
	if n <= 0 {
		return PeerId{}, fmt.Errorf("Unable to read first part of uuid: %d", n)
	}
	part2, n := binary.Varint(b[8:])
	if n <= 0 {
		return PeerId{}, fmt.Errorf("Unable to read second part of uuid: %d", n)
	}
	return PeerId{part1, part2}, nil
}

func randomPeerId() PeerId {
	id, err := readPeerId(uuid.NewRandom())
	if err != nil {
		panic(fmt.Sprintf("Unable to generate random peer id: %s", err))
	}
	return id
}

func (id PeerId) write(b []byte) {
	binary.PutVarint(b, id.part1)
	binary.PutVarint(b[8:], id.part2)
}

func (id PeerId) toBytes() []byte {
	b := make([]byte, PEER_ID_LENGTH)
	id.write(b)
	return b
}

func idFor(msg []byte) (id PeerId, err error) {
	if len(msg) < PEER_ID_LENGTH {
		return PeerId{}, fmt.Errorf("Insufficient data to read peer id, message may be truncated")
	}
	return readPeerId(msg[:PEER_ID_LENGTH])
}

func swapId(msg []byte, newId PeerId) error {
	if len(msg) < PEER_ID_LENGTH {
		return fmt.Errorf("Insufficient room to write peer id")
	}
	newId.write(msg)
	return nil
}
