package server

import (
	"code.google.com/p/go.net/websocket"
)

const channelBufSize = 100

type wsClient struct {
	id          int64
	username    string
	token       string
	ws          *websocket.Conn
	filesToSend chan db.Metadata
	doneCh      chan bool
}

func newWSClient(ws *websocket.Conn, id int64, username string, token string) *wsClient {

	if ws == nil {
		panic("ws cannot be nil")
	}
	filesToSend = make(chan db.Metadata, 100)
	doneCh := make(chan bool)

	return &wsClient{id, username, token, ws, filesToSend, doneCh}
}

func (c *wsClient) conn() *websocket.Conn {
	return c.ws
}

func (c *wsClient) write() {
	select {
	default:
		wsDel(c)
		logger.Errorf("client %d is disconnected.", c.id)
	}
}

func (c *wsClient) sendFile(metadata db.Metadata) {
	c.filesToSend <- metadata

}

func (c *wsClient) done() {
	c.doneCh <- true
}

// Listen Write and Read request via chanel
func (c *wsClient) listen() {
	go c.listenWrite()
	c.listenRead()
}

// Listen write request via chanel
func (c *wsClient) listenWrite() {
	logger.Println("Listening write to client")
	for {
		select {

		// send metadata to the client
		case metadata := <-c.filesToSend:
			log.Println("Send:", metadata)
			websocket.JSON.Send(c.ws, metadata)

		// receive done request
		case <-c.doneCh:
			wsDel(c)
			c.doneCh <- true // for listenRead method
			return
		}
	}
}

// Listen read request via chanel
func (c *wsClient) listenRead() {
	log.Println("Listening read from client")
	for {
		select {

		// receive done request
		case <-c.doneCh:
			wsDel(c)
			c.doneCh <- true // for listenWrite method
			return

		// read data from websocket connection
		default:
			var metadata Metadata
			err := websocket.JSON.Receive(c.ws, &metadata)
			if err == io.EOF {
				c.doneCh <- true
			} else if err != nil {
				logger.Printf(err)
			} else {

			}
		}
	}
}
