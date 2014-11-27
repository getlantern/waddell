package waddell

import (
	"fmt"
)

type topic struct {
	id     TopicId
	client *Client
	out    chan *Message
}

// TopicOut returns the (one and only) channel for writing to the topic
// identified by the given id.
func (c *Client) TopicOut(id TopicId) chan<- *Message {
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

func (t *topic) processOut() {
	for msg := range t.out {
		info := t.client.getConnInfo()
		if info.err != nil {
			log.Errorf("Unable to get connection to waddell, stop sending to %s: %s", t.id, info.err)
			t.client.Close()
			return
		}
		_, err := info.writer.WritePieces(msg.Peer.toBytes(), msg.topic.toBytes(), msg.Body)
		if err != nil {
			t.client.connError(err)
			continue
		}
	}
}

// TopicIn returns the (one and only) channel for receiving from the topic
// identified by the given id.
func (c *Client) TopicIn(id TopicId) <-chan *Message {
	return c.topicIn(id, true)
}

func (c *Client) topicIn(id TopicId, create bool) chan *Message {
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
		topicIn := c.topicIn(msg.topic, false)
		if topicIn != nil {
			topicIn <- msg
		}
	}
}

func (info *connInfo) receive() (*Message, error) {
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
	return &Message{
		Peer:  peer,
		topic: topic,
		Body:  frame[PeerIdLength+TopicIdLength:],
	}, nil
}
