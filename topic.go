package waddell

import (
	"fmt"
)

// Out returns the (one and only) channel for writing to the topic identified by
// the given id.
func (c *Client) Out(id TopicId) chan<- *Message {
	c.topicsOutMutex.Lock()
	defer c.topicsOutMutex.Unlock()
	t := c.topicsOut[id]
	if t == nil {
		t = &topic{
			id:     id,
			client: c,
			out:    make(chan *Message),
		}
		c.topicsOut[id] = t
		go t.processOut()
	}
	return t.out
}

// In returns the (one and only) channel for receiving from the topic identified
// by the given id.
func (c *Client) In(id TopicId) <-chan *Message {
	return c.in(id, true)
}

type topic struct {
	id     TopicId
	client *Client
	out    chan *Message
}

type messageWithTopic struct {
	m *Message

	// topic is the topic to which this message was/will be posted
	topic TopicId
}

func (t *topic) processOut() {
	for msg := range t.out {
		info := t.client.getConnInfo()
		if info.err != nil {
			log.Errorf("Unable to get connection to waddell, stop sending to %s: %s", t.id, info.err)
			t.client.Close()
			return
		}
		_, err := info.writer.WritePieces(msg.Peer.toBytes(), t.id.toBytes(), msg.Body)
		if err != nil {
			t.client.connError(err)
			continue
		}
	}
}

func (c *Client) in(id TopicId, create bool) chan *Message {
	c.topicsInMutex.Lock()
	defer c.topicsInMutex.Unlock()
	ch := c.topicsIn[id]
	if ch == nil && create {
		ch = make(chan *Message)
		c.topicsIn[id] = ch
	}
	return ch
}

func (c *Client) processInbound() {
	for {
		info := c.getConnInfo()
		if info.err != nil {
			log.Errorf("Unable to get connection to waddell, stop receiving: %s", info.err)
			c.Close()
			return
		}
		msg, err := info.receive()
		if err != nil {
			c.connError(err)
			continue
		}
		topicIn := c.in(msg.topic, false)
		if topicIn != nil {
			topicIn <- msg.m
		}
	}
}

func (info *connInfo) receive() (*messageWithTopic, error) {
	log.Trace("Receiving")
	frame, err := info.reader.ReadFrame()
	log.Tracef("Received %d: %s", len(frame), err)
	if err != nil {
		return nil, err
	}
	if len(frame) < WaddellHeaderLength {
		return nil, fmt.Errorf("Frame not long enough to contain waddell headers. Needed %d bytes, found only %d.", WaddellHeaderLength, len(frame))
	}
	peer, err := readPeerId(frame)
	if err != nil {
		return nil, err
	}
	topic, err := readTopicId(frame[PeerIdLength:])
	return &messageWithTopic{
		m: &Message{
			Peer: peer,
			Body: frame[PeerIdLength+TopicIdLength:],
		},
		topic: topic,
	}, nil
}
