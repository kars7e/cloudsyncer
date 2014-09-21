package server

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

	"github.com/gorilla/context"
	"github.com/gorilla/mux"
)

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

// Handler function for metadata action. Returns metadata for given path, optionally for given revision
// If path or revision does not exist, this method returns error.
//
// HTTP codes returned:
//	400 - request invalid (missing parameter, too long, etc.)
//	404 - path or revision does not exist
//	50x - server error processing request
//	200 - Metadata returned
func metadata(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		handleErr(w, 400, err, "Failed to parse form")
		return
	}
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
	if r.FormValue("rev") != "" {
		rev, err := strconv.ParseInt(r.FormValue("rev"), 10, 0)
		if err != nil || rev == 0 {
			handleErr(w, 400, nil, "rev parameter is incorrect")
			return
		}
		revision, err = file.GetRevision(rev)
	} else {
		if file.IsRemoved {
			handleErr(w, 404, nil, "file "+path+" not found")
			return
		}

		revision, err = file.GetCurrentRevision()

		if revision == nil {
			handleErr(w, 404, err, "Revision for given path does not exist")
			return
		}
		if err != nil {
			handleErr(w, 500, err, "Error getting current revision for file: "+file.Path)
			return
		}

	}

	metadata, err := revision.GetMetadata()
	metadataJSON, err := json.Marshal(metadata)
	fmt.Fprintf(w, string(metadataJSON))
	return
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
//	200 - Upload successful
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
	session := context.Get(r, "session").(*db.Session)
	filepath = toolkit.OnlyCleanPath(filepath)
	uuidVal := uuid.New()
	size, err := storage.Store(uuidVal, r.Body)
	if err != nil {
		handleErr(w, 500, err, "Error saving file: "+err.Error())
		return
	}
	hash := storage.GetHash(uuidVal)
	revision, err := user.CreateRevision(filepath, uuidVal, size, hash)
	if err != nil {
		handleErr(w, 500, err, "Error saving revision")
		return
	}

	metadata, err := revision.GetMetadata()
	metadataJSON, err := json.Marshal(metadata)
	fmt.Fprintf(w, string(metadataJSON))
	sendUpdate(user.Id, session.Token)
	sendUpdateWS(user.Id, session.Token, metadata)
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
	if r.FormValue("rev") != "" {
		rev, err := strconv.ParseInt(r.FormValue("rev"), 10, 0)
		if err != nil || rev == 0 {
			handleErr(w, 400, nil, "rev parameter is incorrect")
			return
		}
		revision, err = file.GetRevision(rev)
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
	if err != nil {
		handleErr(w, 500, err, "Error retrieving content for uuid "+revision.Uuid)
		return
	}
	http.ServeContent(w, r, filepath.Base(path), time.Time{}, fileContent)
	return
}

// Handler function for check_upload action. Informs the client whether the file should be uploaded or not,
// by checking its size and hash. If the filepath provided exists and both content and metadata is not changed,
// returns 200 ok. if the content is available but metadata is different, new revision is created,
// and 201 created is returned. If content is not available or filepath does not exist, response is 204 no content.
//
// HTTP codes returned:
//	400 - request invalid (missing parameter, too long, etc.)
//	50x - server error processing request
//	201 - metadata changed, returns updated revision
//	200 - nothing changed, returns the latest revision
//	204 - no content, needs uploading no other response
func check_upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		handleErr(w, 400, err, "Failed to parse form")
		return
	}
	if r.FormValue("filepath") == "" {
		handleErr(w, 400, nil, "filepath not provided")
		return
	}

	if r.FormValue("size") == "" {
		handleErr(w, 400, nil, "size not provided")
		return
	}

	if r.FormValue("hash") == "" {
		handleErr(w, 400, nil, "hash not provided")
		return
	}

	if r.FormValue("name") == "" {
		handleErr(w, 400, nil, "name not provided")
		return
	}

	path := r.FormValue("filepath")
	user := context.Get(r, "user").(*db.User)
	path = toolkit.CleanPath(path)
	file, err := user.GetFileByPath(path)

	if err != nil {
		handleErr(w, 500, err, "Error Creating revision for file "+path)
		return
	}
	if file == nil {
		handleErr(w, 204, nil, "need content")
		return
	}
	var revision *db.Revision
	size, err := strconv.ParseInt(r.FormValue("size"), 10, 0)
	revision, err = file.GetRevisionBySizeAndHash(size, r.FormValue("hash"))
	if revision == nil {
		handleErr(w, 204, nil, "need content")
		return
	}
	if revision.Name != r.FormValue("name") || revision.Id != file.CurrentRevisionId {
		newRevision, err := user.CreateRevision(path, revision.Uuid, revision.Size, revision.Hash)
		if err != nil {
			handleErr(w, 500, err, "Error Creating revision for file "+path)
			return
		}
		metadata, err := newRevision.GetMetadata()
		metadataJSON, err := json.Marshal(metadata)
		w.WriteHeader(201)
		fmt.Fprintf(w, string(metadataJSON))
	}

	metadata, err := revision.GetMetadata()
	metadataJSON, err := json.Marshal(metadata)
	fmt.Fprintf(w, string(metadataJSON))

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
		if err != nil {
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
	resp["cursor"] = strconv.FormatInt(newCursor, 10)
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
	if err != nil {
		handleErr(w, 500, err, "Error reading changes from cursor")
		return
	}

	logger.Debugf("Received %d records in chageset", len(changeSet))
	if len(changeSet) > 0 {
		logger.Debugf("Changes are immediatly available, do not longpoll")
		changes = true
	} else {
		logger.Debug("waiting for new changes to arrive for ", user.Username)
		newChangeArrived[user.Id] = make(map[string](chan int))
		newChangeArrived[user.Id][session.Token] = make(chan int)

		timer := time.NewTimer(time.Second * 60)

		select {
		case _ = <-newChangeArrived[user.Id][session.Token]:
			logger.Debugf("received newChangeArrived signal for user %s and session %s", user.Username, session.Token)
			timer.Stop()
			changes = true
			break
		case _ = <-timer.C:
			logger.Debugf("polling timed out for user %s and session %s", user.Username, session.Token)
			break
		}
	}
	respJSON, err := json.Marshal(map[string]bool{"changes": changes})
	if err != nil {
		handleErr(w, 500, err, "Error marshaling json")
		return
	}
	fmt.Fprintf(w, string(respJSON))
}

// Used to send update to long polling clients.
func sendUpdate(userid id, curToken string) {
	go func() {
		time.Sleep(2000 * time.Millisecond)
		logger.Debug("sendUpdate executed")
		for token, ch := range newChangeArrived[userid] {
			//if curToken != token {
			logger.Debug("Mamy kanal dla sesji ", token)
			if ch != nil {
				ch <- 1
				close(ch)
				delete(newChangeArrived[userid], token)
			}
			//}
		}
	}()
}

func sendUpdateWS(userid id, curToken string, file db.Metadata) {
	for token, client := range wsClients[userid] {
		logger.Debug("Mamy socket dla sesji ", token)
		if client && curToken != token {
			client.sendFile(file)
		}

	}
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
	session := context.Get(r, "session").(*db.Session)
	file, err := user.CreateFolder(toolkit.OnlyCleanPath(r.FormValue("path")))
	if err != nil {
		handleErr(w, 500, err, "Unable to create folder")
		return
	}
	metadata, err := file.GetMetadata(nil)
	metadataJSON, err := json.Marshal(metadata)
	fmt.Fprintf(w, string(metadataJSON))
	sendUpdate(user.Id, session.Token)
	sendUpdateWS(user.Id, session.Token, metadata)
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
	session := context.Get(r, "session").(*db.Session)
	logger.Debugf("received request to remove path: %s", r.FormValue("path"))
	file, err := user.Remove(toolkit.CleanPath(r.FormValue("path")))
	if err != nil {
		handleErr(w, 500, err, "Unable to remove path")
		return
	}
	metadata, err := file.GetMetadata(nil)
	metadataJSON, err := json.Marshal(metadata)
	fmt.Fprintf(w, string(metadataJSON))
	sendUpdate(user.Id, session.Token)
	sendUpdateWS(user.Id, session.Token, metadata)
	return
}
