/*
This package is responsible for handling incoming and outgoing traffic using HTTP or WebSocket Protocol
Serve() method is the entry point which sets up handles for each endpoint
and starts the server listener.
*/
package server

import (
	"cloudsyncer/cs-server/db"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/meatballhat/negroni-logrus"
)

// Logger keeps pointer to the logger struct. It's defined globally for the package scope,
// so all other methods have access to it.
var logger *logrus.Logger

// newChangeArrived keeps channels for notyfing longpolling clients that new change has arrived.
// When new client connects to longpoll_delta endpoint, new channel is created for that particular user and session.
// Methods which cause changes to file structure notify clients by sending some arbitrary data to those channels.
var newChangeArrived map[int]map[string]chan int = make(map[int]map[string]chan int)

// AuthMiddleware checks for credentials for each request that requires authentication.
// Authentication is made by setting HTTP headers. Two headers are required:
//	X-Cloudsyncer-Authtoken - token grabbed from login endpoint
//	X-Cloudsyncer-Username - User name
type AuthMiddleware struct{}

// Middleware serving method. Checks for headers and returns status codes depending on situation:
//	403 - credentials are missing or invalid
//	413 - credentials length is too big
// If no error is given, it passes the execution to the actual endpoint handler
func (l *AuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	token := r.Header.Get("X-Cloudsyncer-Authtoken")
	username := r.Header.Get("X-Cloudsyncer-Username")

	if username == "" {
		handleErr(w, 403, nil, "username not provided or empty")
		return
	}

	if len(username) > 255 {
		handleErr(w, 403, nil, "username too long")
		return
	}

	if token == "" {
		handleErr(w, 403, nil, "Token not provided or empty")
		return
	}

	if len(token) > 255 {
		handleErr(w, 413, nil, "Token too long")
		return
	}
	user := db.GetUser(username)
	if user == nil {
		handleErr(w, 403, nil, "Invalid credentials")
		return
	}
	if session := db.GetSession(user, token); session == nil {
		handleErr(w, 403, nil, "Invalid credentials")
		return
	} else {
		context.Set(r, "session", session)
		context.Set(r, "user", user)
	}
	next(w, r)

}

// Helper type used to marhsal authencity token using json format
type Token struct {
	AuthencityToken string `json:"authencity_token"`
}

// Helper function for easy HTTP error handling. Logs error and writes error status code to the response
func handleErr(w http.ResponseWriter, errorcode int, err error, msg string) {
	if err != nil {
		logger.WithField("error", err.Error()).Error(msg)
	} else {
		logger.Error(msg)
	}
	http.Error(w, "", errorcode)
}

// Sets the logger object
func SetLogger(_logger *logrus.Logger) {
	logger = _logger
}

// Sets AuthMiddleware on endpoint function. Used to add authentication to endpoints that need that.
func authWrapFunc(f func(http.ResponseWriter, *http.Request)) http.Handler {
	return negroni.New(&AuthMiddleware{}, negroni.Wrap(http.HandlerFunc(f)))
}

// Sets AuthMiddleware on endpoint handler. Used to add authentication to endpoints that need that.
func authWrap(h http.Handler) http.Handler {
	return negroni.New(&AuthMiddleware{}, negroni.Wrap(h))
}

// main entry of the package. Initializes handlers, adds middlewares and starts http server
func Serve(address string, port int) error {

	fmt.Printf("start\n")
	router := mux.NewRouter()

	router.HandleFunc("/register", register)
	router.HandleFunc("/login", login)
	router.Handle("/delta", authWrapFunc(delta)).Methods("POST")
	router.Handle("/longpoll_delta", authWrapFunc(longpoll_delta)).Methods("GET")
	router.Handle("/changes", authWrap(wsHandler()))
	router.Handle("/revisions/{filepath:.*}", authWrapFunc(revisions))
	router.Handle("/metadata/{filepath:[^\\/].*}", authWrapFunc(metadata))
	router.Handle("/files/{filepath:.*}", authWrapFunc(file)).Methods("GET")
	router.Handle("/files_put/{filepath:.*}", authWrapFunc(upload)).Methods("PUT")
	router.Handle("/create_folder", authWrapFunc(createFolder)).Methods("POST")
	router.Handle("/remove", authWrapFunc(remove)).Methods("POST")
	router.Handle("/check_upload", authWrapFunc(check_upload)).Methods("POST")
	logMiddleware := negronilogrus.NewMiddleware()
	logMiddleware.Logger = logger
	negroni := negroni.New()
	negroni.Use(logMiddleware)
	negroni.UseHandler(router)
	go wsListen()
	http.ListenAndServe(address+":"+strconv.Itoa(port), context.ClearHandler(negroni))
	return nil
}
