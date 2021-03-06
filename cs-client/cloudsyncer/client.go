package cloudsyncer

import (
	"cloudsyncer/cs-client/db"
	"cloudsyncer/toolkit"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Client is used by other components to perform network calls to server.
// struct Client holds current work dir path, username and authorization token, and pointer to http.Client.
type Client struct {
	path      string
	client    *http.Client
	authToken string
	hostname  string
	cursor    string
	username  string
}

// Creates and returns new instance of Client.
func NewClient(path string) *Client {
	c := Client{path: path}
	c.client = new(http.Client)
	c.hostname = "http://localhost:9999"
	return &c
}

// Check whether client requires login
func (c *Client) NeedLogin() bool {
	return c.authToken == ""
}

// Sets current cursor
func (c *Client) SetCursor(cursor string) {
	c.cursor = cursor
}

// Sets username and authencity token
func (c *Client) SetCredentials(token string, username string) {
	c.authToken = token
	c.username = username
}

// Performs call to remote register endpoint, and registers user. Might return error if something went wrong.
func (c *Client) Register(username string, password string, computername string) (authToken string, err error) {
	serverUrl := c.hostname + "/register"
	data := url.Values{}
	data.Set("username", username)
	data.Add("password", password)
	data.Add("computername", computername)
	response, err := c.client.PostForm(serverUrl, data)
	if err != nil {
		log.Print("error on register: ", err)
		return
	}
	defer response.Body.Close()
	log.Print("received: ", response.Status, " ", err)
	if response.StatusCode != http.StatusOK {
		log.Print("register failed: ", response.Status)
		return
	}
	decoder := json.NewDecoder(response.Body)
	token := new(Token)
	if err = decoder.Decode(token); err != nil {
		log.Print("error decoding response: ", err)
		return
	}
	c.authToken = token.AuthencityToken
	c.username = username
	authToken = c.authToken
	err = nil
	return
}

// Logs in  user with given password and computer name. Might return error if something went wrong.
func (c *Client) Login(username string, password string, computername string) (authToken string, err error) {
	serverUrl := c.hostname + "/login"
	data := url.Values{}
	data.Set("username", username)
	data.Add("password", password)
	data.Add("computername", computername)
	response, err := c.client.PostForm(serverUrl, data)
	if err != nil {
		log.Print("error on login: ", err)
		return
	}
	defer response.Body.Close()
	log.Print("received: ", response.Status, " ", err)
	if response.StatusCode != http.StatusOK {
		log.Print("login failed: ", response.Status)
		err = errors.New("received wrong status code: " + response.Status)
		return
	}
	decoder := json.NewDecoder(response.Body)
	token := new(Token)
	if err = decoder.Decode(token); err != nil {
		log.Print("error decoding response: ", err)
		return
	}
	c.authToken = token.AuthencityToken
	c.username = username
	authToken = c.authToken
	err = nil
	return
}

func (c *Client) setAuth(header http.Header) {
	//log.Printf("setting username: '%s' and token '%s'", c.username, c.authToken)
	header.Set("X-Cloudsyncer-Authtoken", c.authToken)
	header.Set("X-Cloudsyncer-Username", c.username)
}

// Uploads file with given Metadata. Used by Worker.
func (c *Client) Upload(path string) (db.Metadata, error) {
	if !strings.HasPrefix(path, c.path) {
		log.Printf("file '%s' does not have valid prefix '%s'", path, c.path)
		return db.Metadata{}, os.ErrInvalid
	}

	file, err := os.Open(path)
	if err != nil {
		log.Printf("error opening file '%s'", path)
		return db.Metadata{}, err
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		log.Printf("error stating file '%s'", path)
		return db.Metadata{}, err
	}
	if fi.IsDir() {
		log.Print("we need file, not directory")
		return db.Metadata{}, os.ErrInvalid
	}
	relativePath := strings.Replace(path, c.path, "", 1)
	if path == "" {
		log.Print("we cannot upload main directory!")
		return db.Metadata{}, os.ErrInvalid
	}
	serverUrl := c.hostname + "/files_put" + toolkit.OnlyCleanPath(strings.Replace(relativePath, `\`, "/", -1))
	req, err := http.NewRequest("PUT", serverUrl, file)
	if err != nil {
		return db.Metadata{}, err
	}
	c.setAuth(req.Header)
	req.Header.Set("Content-Length", string(fi.Size()))
	resp, err := c.client.Do(req)
	if err != nil {
		return db.Metadata{}, err
	}
	if resp.StatusCode == 200 {

		metadata := db.Metadata{}
		rawJson, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(rawJson, &metadata)
		if err != nil {
			return db.Metadata{}, err
		}
		return metadata, nil
	}
	return db.Metadata{}, errors.New("received wrong status: " + resp.Status)
}

// Creates directory with given metadata on server. Used by Worker.
func (c *Client) Mkdir(path string) (db.Metadata, error) {
	if !strings.HasPrefix(path, c.path) {
		log.Printf("file '%s' does not have valid prefix '%s'", path, c.path)
		return db.Metadata{}, os.ErrInvalid
	}

	relativePath := strings.Replace(path, c.path, "", 1)
	if path == "" {
		log.Print("we cannot upload main directory!")
		return db.Metadata{}, os.ErrInvalid
	}
	serverUrl := c.hostname + "/create_folder"
	data := url.Values{}
	data.Set("path", toolkit.OnlyCleanPath(strings.Replace(relativePath, `\`, "/", -1)))
	body := strings.NewReader(data.Encode())
	req, err := http.NewRequest("POST", serverUrl, body)
	if err != nil {
		return db.Metadata{}, err
	}
	c.setAuth(req.Header)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return db.Metadata{}, err
	}
	if resp.StatusCode == 200 {
		metadata := db.Metadata{}
		rawJson, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(rawJson, &metadata)
		if err != nil {
			return db.Metadata{}, err
		}
		return metadata, nil
	}
	return db.Metadata{}, errors.New("received wrong status: " + resp.Status)
}

// Removes file with given path from server. Used by Worker.
func (c *Client) Remove(path string) error {
	if !strings.HasPrefix(path, c.path) {
		log.Printf("file '%s' does not have valid prefix '%s'", path, c.path)
		return os.ErrInvalid
	}

	relativePath := strings.Replace(path, c.path, "", 1)
	if path == "" {
		log.Print("we cannot remove main directory!")
		return os.ErrInvalid
	}
	serverUrl := c.hostname + "/remove"
	data := url.Values{}
	data.Set("path", relativePath)
	body := strings.NewReader(data.Encode())
	req, err := http.NewRequest("POST", serverUrl, body)
	if err != nil {
		return err
	}
	c.setAuth(req.Header)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode == 200 {
		return nil
	}
	return errors.New("received wrong status: " + resp.Status)
}

// Long polls server for new changes. Used by Listener.
func (c *Client) Poll(cursor string) (changes bool, err error) {
	serverUrl := c.hostname + "/longpoll_delta"
	data := url.Values{}
	data.Set("cursor", cursor)
	serverUrl += "?" + data.Encode()
	req, err := http.NewRequest("GET", serverUrl, nil)
	if err != nil {
		return false, err
	}
	c.setAuth(req.Header)
	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == 200 {
		jsonResponse := make(map[string]bool)
		rawJson, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(rawJson, &jsonResponse)
		if err != nil {
			return false, err
		}
		return jsonResponse["changes"], nil
	}
	return false, errors.New("received wrong status: " + resp.Status)
}

// Retrieves file with given path and given revision from server. Used by worker.
func (c *Client) GetFile(path string, rev string) (io.ReadCloser, error) {

	serverUrl := c.hostname + "/files" + path
	data := url.Values{}
	data.Set("rev", rev)
	serverUrl += "?" + data.Encode()
	req, err := http.NewRequest("GET", serverUrl, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req.Header)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 200 {
		return resp.Body, nil
	}
	return nil, errors.New("received wrong status: " + resp.Status)

}

// Retrieves delta from given cursor. Used by listener.
func (c *Client) GetDelta(cursor string) (Delta, error) {
	serverUrl := c.hostname + "/delta"
	data := url.Values{}
	data.Set("cursor", cursor)
	body := strings.NewReader(data.Encode())
	req, err := http.NewRequest("POST", serverUrl, body)
	if err != nil {
		return Delta{}, err
	}
	c.setAuth(req.Header)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return Delta{}, err
	}
	if resp.StatusCode == 200 {
		delta := Delta{}
		rawJson, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(rawJson, &delta)
		if err != nil {
			return Delta{}, err
		}
		return delta, nil
	}
	return Delta{}, errors.New("received wrong status: " + resp.Status)
}
