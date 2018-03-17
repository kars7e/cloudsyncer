package cloudsyncer

import (
	"log"
	"time"
)

// Listener is responsible for retrieveing changes from remote server.
// Whenever new change arrives, its being sent by listener to deltas channel.
// uses client to attach to server.
type Listener struct {
	deltas chan Delta
	cursor string
	client *Client
}

// Creates and returns new Listener with given parameters.
func NewListener(deltas chan Delta, client *Client) *Listener {
	l := Listener{deltas: deltas, client: client}
	return &l
}

// sets current cursor and starts long polling for new changes.
// When new changes arrive executes delta method. If no changes arrive in given time server timeouts
// and poll executes itself again.
func (l *Listener) Listen(cursor string) {
	l.cursor = cursor
	go l.poll()
}

// sets current cursor and starts long polling for new changes.
// When new changes arrive executes delta method. If no changes arrive in given time server timeouts
// and poll executes itself again.
func (l *Listener) Listen(cursor string) {
	l.cursor = cursor
	go l.poll()
}

func (l *Listener) ListenWS(cursor string) {
	l.cursor = cursor
	c.startWS()
	go l.poll()
}

func (l *Listener) poll() {
	for {
		curCursor := l.cursor
		log.Print("polling for new changes from cursor " + curCursor)
		changes, err := l.client.Poll(curCursor)
		if err != nil {
			log.Print("Error when polling for changes")
			return
		}
		if curCursor != l.cursor {
			log.Printf("poller discarded, cursor changed during polling")
			return
		}
		if changes == true {
			log.Print("new changes arrived, executing delta()")
			l.delta()
			return
		}
		log.Print("no new changes, polling again")
	}
}

func (l *Listener) delta() {
	delta, err := l.client.GetDelta(l.cursor)
	if err != nil {
		log.Printf("failed to get delta for cursor '%s', trying again in 3 secs, %s", l.cursor, err)
		time.Sleep(3 * time.Second)
		delta, err = l.client.GetDelta(l.cursor)
		if err != nil {
			log.Printf("failed to get delta for cursor '%s', giving up: %s", l.cursor, err)
			return
		}
	}
	logger.Debugf("Received delta: %v", delta)
	l.deltas <- delta
	logger.Debugf("Setting cursor to: %s", delta.Cursor)
	l.cursor = delta.Cursor
	go l.poll()
	return
}
