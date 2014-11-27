package waddell

import (
	"fmt"
	"net"
	"time"

	"github.com/getlantern/framed"
)

type connInfo struct {
	id     PeerId
	conn   net.Conn
	reader *framed.Reader
	writer *framed.Writer
	err    error
}

func (c *Client) stayConnected() {
	var info *connInfo
	for {
		select {
		case err := <-c.connErrCh:
			log.Tracef("Encountered error, disconnecting: %s", err)
			if info != nil {
				info.conn.Close()
				info = nil
			}
		case infoCh, open := <-c.connInfoChs:
			if !open {
				log.Trace("connInfoChs closed, done processing")
				return
			}
			if info != nil {
				infoCh <- info
				continue
			}
			info = c.connect()
			infoCh <- info
		}
	}
}

func (c *Client) connect() *connInfo {
	log.Trace("Connecting ...")
	var lastErr error
	consecutiveFailures := 0
	for {
		if consecutiveFailures > c.ReconnectAttempts {
			log.Tracef("Done trying to connect: %s", lastErr)
			return &connInfo{
				err: fmt.Errorf("Unable to connect: %s", lastErr),
			}
		}
		delay := time.Duration(consecutiveFailures) * reconnectDelayInterval
		if delay > maxReconnectDelay {
			delay = maxReconnectDelay
		}
		log.Tracef("Waiting %s before dialing", delay)
		time.Sleep(delay)
		info, err := c.connectOnce()
		if err == nil {
			return info
		}
		log.Tracef("Unable to connect: %s", err)
		lastErr = err
		info = nil
		consecutiveFailures += 1
	}
}

func (c *Client) connectOnce() (*connInfo, error) {
	conn, err := c.Dial()
	if err != nil {
		return nil, err
	}
	info := &connInfo{
		conn:   conn,
		reader: framed.NewReader(conn),
		writer: framed.NewWriter(conn),
	}
	// Read first message to get our PeerId
	msg, err := info.receive()
	if err != nil {
		return nil, fmt.Errorf("Unable to get peerid: %s", err)
	}
	info.id = msg.From
	if c.IdCallback != nil {
		c.IdCallback(info.id)
	}
	return info, nil
}

func (c *Client) connError(err error) {
	c.connErrCh <- err
}

func (c *Client) getConnInfo() *connInfo {
	infoCh := make(chan *connInfo)
	c.connInfoChs <- infoCh
	return <-infoCh
}
