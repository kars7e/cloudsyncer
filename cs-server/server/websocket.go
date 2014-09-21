package server

import (
	"cloudsyncer/cs-server/db"
	"code.google.com/p/go.net/websocket"
	"github.com/gorilla/context"
	"net/http"
)

var wsClients map[int]map[string]*wsClient
var wsAddCh chan *wsClient
var wsDelCh chan *wsClient
var wsDoneCh chan bool
var wsErrCh chan error

func wsAdd(c *wsClient) {
	wsAddCh <- c
}

func wsDel(c *wsClient) {
	wsDelCh <- c
}

func wsDone() {
	wsDoneCh <- true
}

func wsErr(err error) {
	wsErrCh <- err
}

// Listen and serve.
// It serves client connection and broadcast request.
func wsListen() {
	wsClients := make(map[int]map[string]*wsClient)
	wsAddCh := make(chan *wsClient)
	wsDelCh := make(chan *wsClient)
	wsDoneCh := make(chan bool)
	wsErrCh := make(chan error)
	logger.Println("Listening websocket server...")

	for {
		select {

		// Add new a client
		case c := <-wsAddCh:
			logger.Println("Added new client")

		// del a client
		case c := <-wsDelCh:
			logger.Println("Delete client")

		case err := <-wsErrCh:
			logger.Println("Error:", err.Error())

		case <-wsDoneCh:
			return
		}
	}
}

func changes(ws *websocket.Conn) {
	defer func() {
		err := ws.Close()
		if err != nil {
			logger.Printf("asd")
		}
	}()
	user := context.Get(ws.Request(), "user").(*db.User)
	session := context.Get(ws.Request(), "session").(*db.Session)
	client := newWSClient(ws, user.Id, user.Username, session.Token)
	wsAdd(client)
	client.listen()
}

func wsHandler() http.Handler {
	return websocket.Handler(changes)
}
