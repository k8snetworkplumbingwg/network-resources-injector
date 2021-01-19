package webhook

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	interval = time.Millisecond * 2
	chBuffer = 1
)

//Channel contains fields to safely manage a channel
type Channel struct {
	ch     chan struct{}
	isOpen bool
	mutex  sync.Mutex
}

//NewChannel returns an instance of type Channel
func NewChannel() *Channel {
	return &Channel{}
}

//Close checks if channel closed before trying to close
func (c *Channel) Close() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.isOpen {
		close(c.ch)
		c.isOpen = false
	}
}

//Open opens a channel
func (c *Channel) Open() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.ch = make(chan struct{}, chBuffer)
	c.isOpen = true
}

//GetCh returns channel
func (c *Channel) GetCh() chan struct{} {
	return c.ch
}

//IsOpen checks if channel is open
func (c *Channel) IsOpen() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.isOpen
}

//IsClosed checks if channel closed
func (c *Channel) IsClosed() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return !c.isOpen
}

//WaitUntilClosed will block until time limit is reached or channel is closed
func (c *Channel) WaitUntilClosed(limit time.Duration) error {
	if interval > limit {
		return errors.New("limit arg value too low")
	}
	tEnd := time.Now().Add(limit)
	for tEnd.After(time.Now()) {
		if c.IsClosed() {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("timed out after waiting '%s'", limit.String())
}

//WaitUntilOpened will block until time limit is reached or channel is opened
func (c *Channel) WaitUntilOpened(limit time.Duration) error {
	if interval > limit {
		return errors.New("limit arg value too low")
	}
	tEnd := time.Now().Add(limit)
	for tEnd.After(time.Now()) {
		if c.IsOpen() {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("timed out after waiting '%s'", limit.String())
}
