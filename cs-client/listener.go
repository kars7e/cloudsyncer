package main

import (
	"log"
	"time"
)

type Listener struct {
	deltas chan Delta
	cursor string
	client *Client
}

func NewListener(deltas chan Delta, client *Client) *Listener {
	l := Listener{deltas: deltas, client: client}
	return &l
}

func (l *Listener) Listen(cursor string) {
	l.cursor = cursor
	go l.poll()
}
func (l *Listener) poll() {
	for {
		changes, err := l.client.Poll(l.cursor)
		if err != nil {
			log.Print("Error when polling for changes")
			return
		}
		if changes == true {
			l.delta()
			return
		}
	}
}

func (l *Listener) delta() {
	delta, err := l.client.GetDelta(l.cursor)
	if err != nil {
		log.Printf("failed to get delta for cursor '%s', trying again in 3 secs", l.cursor)
		time.Sleep(3 * time.Second)
		delta, err = l.client.GetDelta(l.cursor)
		if err != nil {
			log.Printf("failed to get delta for cursor '%s', giving up", l.cursor)
			return
		}
	}
	logger.Debugf("Received delta: %v", delta)
	l.deltas <- *delta
	l.cursor = delta.Cursor
	go l.poll()
	return
}
