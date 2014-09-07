/*
This package handles incoming traffic using HTTP Protocol
RunInterface() method is the entry point which sets up handles for each endpoint
and starts the server listener.
*/
package http

import (
	"cloudsyncer/cs-server/db"
	"cloudsyncer/cs-server/storage"
	"cloudsyncer/toolkit"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"code.google.com/p/go-uuid/uuid"

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
var newChangeArrived map[*db.User]map[*db.Session]chan int = make(map[*db.User]map[*db.Session]chan int)

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

// Handler function for register action.
// Requires the following form parameters:
//	username - username
//	password - password
//	computername (optional) - computer name means client wants to create a session. In that case authenicty_token is returned and login is not required.
//
// HTTP codes returned:
//	400 - request invalid (missing parameter, too long, etc.)
//	413 - password too long (possible DoS attempt)
//	409 - user already exists
//	50x - server error processing request
//	200 - Registration successful
func register(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		handleErr(w, 400, err, "unable to parse form in register action")
		return
	}
	var params = r.Form
	if params["username"] == nil {
		handleErr(w, 400, nil, "username not provided or empty")
		return
	}
	var username = params["username"][0]
	if username == "" {
		handleErr(w, 400, nil, "username not provided or empty")
		return
	}

	if len(username) > 255 {
		handleErr(w, 400, nil, "username too long")
		return
	}

	if params["password"] == nil {
		handleErr(w, 400, nil, "password not provided or empty")
		return
	}
	var password = params["password"][0]
	if password == "" {
		handleErr(w, 400, nil, "Password not provided or empty")
		return
	}

	if len(password) > 255 {
		handleErr(w, 413, nil, "Password too long (possible DoS)")
		return
	}
	var computername string
	if params["computername"] != nil {
		computername = params["computername"][0]
	}
	if user := db.GetUser(username); user != nil {
		handleErr(w, 409, nil, "User already exist")
		return
	}
	user, err := db.CreateUser(username, password)
	if err != nil {
		handleErr(w, 500, err, "Error during user creation")
		return
	}
	if computername != "" {
		session, err := db.CreateSession(user, computername)
		jsonToken, err := json.Marshal(Token{AuthencityToken: session.Token})
		if err != nil {
			handleErr(w, 500, err, "Error on marshalling token for user during register for user "+username+" and computername "+computername)
			return
		}
		fmt.Fprintf(w, string(jsonToken))
	}
	return
}

// Handler function for login action.
// Requires the following form parameters:
//	username - username
//	password - password
//	computername -computer name
//
// If successful, returns authencity token to be used with further requests
//
// HTTP codes returned:
//	400 - request invalid (missing parameter, too long, etc.)
//	413 - password too long (possible DoS attempt)
//	409 - user already exists
//	50x - server error processing request
//	200 - Registration successful
func login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		handleErr(w, 400, err, "unable to parse form in register action")
		return
	}
	var params = r.Form
	if params["username"] == nil {
		handleErr(w, 400, nil, "username not provided or empty")
		return
	}
	var username = params["username"][0]
	if username == "" {
		handleErr(w, 400, nil, "username not provided or empty")
		return
	}

	if len(username) > 255 {
		handleErr(w, 400, nil, "username too long")
		return
	}

	if params["password"] == nil {
		handleErr(w, 400, nil, "password not provided or empty")
		return
	}
	var password = params["password"][0]
	if password == "" {
		handleErr(w, 400, nil, "Password not provided or empty")
		return
	}

	if len(password) > 255 {
		handleErr(w, 413, nil, "Password too long (possible DoS)")
		return
	}
	user := db.GetUser(username)
	if user == nil {
		handleErr(w, 403, nil, "User does not exist")
		return
	}

	if !user.CheckPassword(password) {
		handleErr(w, 403, nil, "Wrong password for user "+username)
		return
	}
	if params["computername"] == nil {
		handleErr(w, 400, nil, "computername not provided or empty")
		return
	}
	var computername = params["computername"][0]
	if computername == "" {
		handleErr(w, 400, nil, "Missing computername for user "+username)
		return
	}

	session, err := db.CreateSession(user, computername)
	jsonToken, err := json.Marshal(Token{AuthencityToken: session.Token})
	if err != nil {
		handleErr(w, 500, err, "Error on marshalling token for user during register for user "+username+" and computername "+computername)
		return
	}
	fmt.Fprintf(w, string(jsonToken))

}

// Handler function for upload action.
// File path to upload should be provided as part of the request URL
// If successful, returns metadata of uploaded file.
// Upload assumes that client knows what he is doing, in particular if the file in given path already exists,
// this method overwrites it.
// If parent directory does not exist, this method returns error.
//
// HTTP codes returned:
//	400 - request invalid (missing parameter, too long, etc.)
//	50x - server error processing request
//	200 - Registration successful
func upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		handleErr(w, 400, err, "Failed to parse form")
		return
	}
	vars := mux.Vars(r)
	if vars["filepath"] == "" {
		handleErr(w, 400, nil, "filepath not provided")
		return
	}
	filepath := "/" + vars["filepath"]
	user := context.Get(r, "user").(*db.User)
	filepath = toolkit.OnlyCleanPath(filepath)
	uuidVal := uuid.New()
	size, err := storage.Store(uuidVal, r.Body)
	if err != nil {
		handleErr(w, 500, err, "Error saving file: "+err.Error())
		return
	}
	revision, err := user.CreateRevision(filepath, uuidVal, size)
	if err != nil {
		handleErr(w, 500, err, "Error saving revision")
		return
	}
	sendUpdate(user)
	metadata, err := revision.GetMetadata()
	metadataJSON, err := json.Marshal(metadata)
	fmt.Fprintf(w, string(metadataJSON))
	return
}

// Handler function for download action.
// Filepath to download should be provided as part of the request URL
// If successful, returns body of the file (metadata might be requested in separate call to metadata endpoint).
// Optional form parameter "rev" might be provided to download particular revision of the file.
//
// HTTP codes returned:
//	400 - request invalid (missing parameter, too long, etc.)
//	404 - file does not exist or is deleted and revision number is not valid
//	50x - server error processing request
//	200 - File exist
func file(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		handleErr(w, 400, err, "Failed to parse form")
		return
	}
	params := r.Form
	vars := mux.Vars(r)

	if vars["filepath"] == "" {
		handleErr(w, 400, nil, "filepath not provided")
		return
	}
	path := "/" + vars["filepath"]
	user := context.Get(r, "user").(*db.User)
	path = toolkit.CleanPath(path)
	file, err := user.GetFileByPath(path)
	if file == nil {
		handleErr(w, 404, nil, "file "+path+" not found")
		return
	}
	var revision *db.Revision
	if params["rev"] != nil && params["rev"][0] != "" {
		rev, err := strconv.ParseInt(params["rev"][0], 10, 0)
		if err != nil || rev == 0 {
			handleErr(w, 400, nil, "rev parameter is incorrect")
			return
		}
		revision, err = user.GetRevision(file, rev)
	} else {
		if file.IsRemoved {
			handleErr(w, 404, nil, "file "+path+" not found")
			return
		}

		revision, err = file.GetCurrentRevision()
		if err != nil {
			handleErr(w, 500, err, "Error getting current revision for file: "+file.Path)
			return
		}

	}
	fileContent, err := storage.Retrieve(revision.Uuid)
	http.ServeContent(w, r, filepath.Base(path), time.Time{}, fileContent)
	return
}

// Handler function for delta action. Returns changes form the given cursor, or full state if cursor is not given.
// If successful, returns list of changes in format:
//	[<filepath>, <metadata>]
// If <metadata> is null, it means that filepath has been removed.
// Optional form parameter "cursor" might be provided to get changes only from the given cursor.
// It also returns the new cursor, which should be used for further requests to delta.
// It might not return any changes, if no changes happened from given cursor.
//
// HTTP codes returned:
//	400 - request invalid (missing parameter, too long, etc.)
//	50x - server error processing request
//	200 - Request succesful
func delta(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		handleErr(w, 400, err, "Failed to parse form")
		return
	}
	user := context.Get(r, "user").(*db.User)
	reset := false
	var newCursor int64
	var state []map[string]interface{}
	var err error
	if r.FormValue("cursor") == "" {
		logger.Debug("No cursor provided, sending full state")
		state, err = user.GetCurrentState()
		if err != nil {
			handleErr(w, 500, err, "Unable to get full state for a user: "+user.Username)
			return
		}
		reset = true
	} else {

		cursor, err := strconv.ParseInt(r.FormValue("cursor"), 10, 0)
		if err != nil || cursor == 0 {
			handleErr(w, 400, nil, "cursor parameter is incorrect")
			return
		}
		state, newCursor, err = user.GetChangesFromCursor(cursor)
		if err != nil {
			handleErr(w, 500, err, "Unable to get changes for user: "+user.Username)
			return
		}
	}
	resp := make(map[string]interface{})
	resp["reset"] = reset
	resp["cursor"] = newCursor
	resp["entries"] = state
	respJSON, err := json.Marshal(resp)
	if err != nil {
		handleErr(w, 500, err, "Error marshaling JSON")
		return
	}
	logger.Debug("sending response: ", string(respJSON))
	fmt.Fprintf(w, string(respJSON))
	return
}

// Handler function for longpoll_delta action. Used to long polling server for new changes.
// returns boolean value "changes". If changes appeared during polling, response is returned immediately with "changes" set to true.
// If no changes appeared during arbitrary period of time (30 seconds), timeout is fired and response is returned with "changes" set to false.
// In both cases 200 code is returned.
// Client should renew the request.
//
// HTTP codes returned:
//	400 - request invalid (missing parameter, too long, etc.)
//	200 - Request succesful
func longpoll_delta(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		handleErr(w, 400, err, "Failed to parse form")
		return
	}
	session := context.Get(r, "session").(*db.Session)
	user := context.Get(r, "user").(*db.User)
	if r.FormValue("cursor") == "" {
		handleErr(w, 400, nil, "Missing required parameter cursor")
		return
	}
	changes := false
	cursor, _ := strconv.ParseInt(r.FormValue("cursor"), 10, 0)
	changeSet, _, err := user.GetChangesFromCursor(cursor)
	if len(changeSet) > 0 {
		changes = true
	} else {
		newChangeArrived[user][session] = make(chan int)

		defer delete(newChangeArrived[user], session)
		defer close(newChangeArrived[user][session])
		timer := time.NewTimer(time.Second * 30)
		<-timer.C

		select {
		case _ = <-newChangeArrived[user][session]:
			timer.Stop()
			changes = true
			break
		case _ = <-timer.C:
			break
		}
		delete(newChangeArrived[user], session)
	}
	respJSON, err := json.Marshal(map[string]bool{"changes": changes})
	if err != nil {
		handleErr(w, 500, err, "Error marshaling json")
		return
	}
	fmt.Fprintf(w, string(respJSON))
}

// Handler function for revisions action. Used to list all revisions available for given filepath.
// Filepath should be provided as a part of the URL.
// Returns list of revisions metadata for given file, if file exists and revisions exist
//
// HTTP codes returned:
//	400 - request invalid (missing parameter, too long, etc.)
//	404 - file does not exist
//	200 - Request succesful
func revisions(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		handleErr(w, 400, err, "Failed to parse form")
		return
	}
	vars := mux.Vars(r)
	filepath := vars["filepath"]
	if filepath == "" {
		handleErr(w, 400, nil, "filepath not provided")
		return
	}
	user := context.Get(r, "user").(*db.User)
	file, err := user.GetFileByPath(filepath)
	if file == nil && err == nil {
		handleErr(w, 404, nil, "file does not exist")
		return
	}
	if err != nil {
		handleErr(w, 500, err, "error getting file")
		return
	}
	revisions, err := file.GetRevisions()
	revsToReturn := make([]map[string]interface{}, len(revisions))
	for index, revision := range revisions {
		revsToReturn[index]["revision"] = index + 1
		revsToReturn[index]["rev"] = revision.Id
		revsToReturn[index]["size"] = revision.Size
		revsToReturn[index]["path"] = filepath
		revsToReturn[index]["name"] = revision.Name
		revsToReturn[index]["modified"] = revision.Updated
		revsToReturn[index]["is_dir"] = revision.IsDir
		if revision.Id == file.CurrentRevisionId {
			revsToReturn[index]["current"] = true
		} else {
			revsToReturn[index]["current"] = false
		}
	}
	revisionsJson, err := json.Marshal(revsToReturn)
	if err != nil {
		handleErr(w, 500, err, "error marshaling json")
		return
	}
	fmt.Fprintf(w, string(revisionsJson))
}

// Handler function for create_folder action.
// Folder path to create should be provided as form parameter "path"
// If successful, returns metadata of created folder.
// To create subdirectory, the parent directory already exists.
//
// HTTP codes returned:
//	400 - request invalid (missing parameter, too long, etc.)
//	50x - server error processing request
//	200 - Registration successful
func createFolder(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		handleErr(w, 400, err, "Failed to parse form")
		return
	}
	if r.FormValue("path") == "" {
		handleErr(w, 400, nil, "path not provided")
		return
	}
	user := context.Get(r, "user").(*db.User)
	file, err := user.CreateFolder(toolkit.OnlyCleanPath(r.FormValue("path")))
	if err != nil {
		handleErr(w, 500, err, "Unable to create folder")
		return
	}
	metadata, err := file.GetMetadata(nil)
	metadataJSON, err := json.Marshal(metadata)
	fmt.Fprintf(w, string(metadataJSON))
	sendUpdate(user)
	return
}

// Handler function for remove action. Used to remove file and folders. If folder is given, all children are removed as well.
// File path to remove should be provided as form parameter "path"
// If successful, returns metadata of removed file/folder.
//
//
// HTTP codes returned:
//	400 - request invalid (missing parameter, too long, etc.)
//	50x - server error processing request
//	200 - Registration successful

func remove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		handleErr(w, 400, err, "Failed to parse form")
		return
	}
	if r.FormValue("path") == "" {
		handleErr(w, 400, nil, "path not provided")
		return
	}
	user := context.Get(r, "user").(*db.User)
	logger.Debugf("received request to remove path: %s", r.FormValue("path"))
	file, err := user.Remove(toolkit.CleanPath(r.FormValue("path")))
	if err != nil {
		handleErr(w, 500, err, "Unable to remove path")
		return
	}
	metadata, err := file.GetMetadata(nil)
	metadataJSON, err := json.Marshal(metadata)
	fmt.Fprintf(w, string(metadataJSON))
	sendUpdate(user)
	return
}

// Sets the logger object
func SetLogger(_logger *logrus.Logger) {
	logger = _logger
}

// Sets AuthMiddleware on endpoint. Used to add authentication to endpoints that need that.
func authWrap(f func(http.ResponseWriter, *http.Request)) http.Handler {
	return negroni.New(&AuthMiddleware{}, negroni.Wrap(http.HandlerFunc(f)))
}

// Used to send update to long polling clients.
func sendUpdate(user *db.User) {
	for _, ch := range newChangeArrived[user] {
		if ch != nil {
			ch <- 1
		}
	}
}

// main entry of the package. Initializes handlers, adds middlewares and starts http server
func RunInterface(address string, port int) error {

	fmt.Printf("start\n")
	router := mux.NewRouter()

	router.HandleFunc("/register", register)
	router.HandleFunc("/login", login)
	router.Handle("/delta", authWrap(delta)).Methods("POST")
	router.Handle("/longpoll_delta", authWrap(longpoll_delta)).Methods("GET")

	router.Handle("/revisions/{filepath:.*}", authWrap(revisions))
	//	router.Handle("/metadata/{filepath:[^\\/].*}", authWrap(metadata))
	router.Handle("/files/{filepath:.*}", authWrap(file)).Methods("GET")
	router.Handle("/files_put/{filepath:.*}", authWrap(upload)).Methods("PUT")
	router.Handle("/create_folder", authWrap(createFolder)).Methods("POST")
	router.Handle("/remove", authWrap(remove)).Methods("POST")
	logMiddleware := negronilogrus.NewMiddleware()
	logMiddleware.Logger = logger
	negroni := negroni.New()
	negroni.Use(logMiddleware)
	//negroni.Use(&DumpMiddleware{})
	negroni.UseHandler(router)
	http.ListenAndServe(address+":"+strconv.Itoa(port), context.ClearHandler(negroni))
	return nil
}
