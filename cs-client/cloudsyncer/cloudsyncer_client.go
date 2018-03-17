package cloudsyncer

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"

	"strings"

	"cloudsyncer/cs-client/db"
	"cloudsyncer/toolkit"

	"github.com/howeyc/gopass"
)

// struct Delta holds information retrieved from delta remote call.
type Delta struct {
	Reset   bool
	Entries []map[string]*db.Metadata
	Cursor  string
}

// authencity token retrieved from server during login remote call.
type Token struct {
	AuthencityToken string `json:"authencity_token"`
}

var appConfig = make(map[string]string)
var discard = make(map[string]bool)

func register(client *Client) (username string, password string, computername string, err error) {
	fmt.Println("Please provide your user data")
	username, password, computername = getLoginAndPassword()
	_, err = client.Register(username, password, computername)
	return username, password, computername, err
}

func getLoginAndPassword() (username string, password string, computername string) {
	reader := bufio.NewReader(os.Stdin)
	for username == "" {
		fmt.Print("\nEnter username: ")
		username, _ = reader.ReadString('\n')
	}
	username = strings.TrimSpace(username)

	for password == "" {
		fmt.Print("\nEnter password: ")
		password = string(gopass.GetPasswdMasked())
	}

	for computername == "" {
		fmt.Print("\nEnter computer name: ")
		computername, _ = reader.ReadString('\n')
	}
	computername = strings.TrimSpace(computername)
	return
}

func prepareDatabase() (err error) {
	if !toolkit.IsDirectory(getConfigFileDir()) {
		err = os.Mkdir(getConfigFileDir(), 0770)
		if err != nil {
			log.Print("Unable to create config file ", getConfigFileDir())
			return err
		}
		log.Print("Created config dir")
	}
	err = db.InitDb(getDbFilePath(), logger)
	return err
}

func initialConfig(client *Client) error {
	appConfig["username"] = db.GetCfgValue("username")
	appConfig["authencity_token"] = db.GetCfgValue("authencity_token")
	appConfig["computer_name"] = db.GetCfgValue("computer_name")
	if appConfig["username"] == "" || appConfig["authencity_token"] == "" || appConfig["computer_name"] == "" {
		username, authencity_token, computer_name, err := loginOrRegister(client)
		if err != nil {
			return err
		}
		appConfig["username"] = username
		appConfig["authencity_token"] = authencity_token
		appConfig["computer_name"] = computer_name
		db.SetCfgValue("username", username)
		db.SetCfgValue("authencity_token", authencity_token)
		db.SetCfgValue("computer_name", computer_name)
	}
	return nil
}
func setupWorkDir() (string, error) {
	fmt.Print("\nEnter cloudsyncer path [", getHomeDir(), string(os.PathSeparator), "cloudsync", "]")
	reader := bufio.NewReader(os.Stdin)
	path, _ := reader.ReadString('\n')
	path = strings.TrimSpace(path)
	if path == "" {
		path = getHomeDir() + string(os.PathSeparator) + "cloudsync"
	}
	if toolkit.IsDirectory(path) {
		return path, nil
	}
	if toolkit.Exists(path) || os.MkdirAll(path, 0777) != nil {
		fmt.Println("Error creating a directory. Please choose different path.")
		return setupWorkDir()
	}
	return path, nil
}
func loginOrRegister(client *Client) (username string, token string, computername string, err error) {
	choice := ""
	password := ""
	reader := bufio.NewReader(os.Stdin)
	for choice != "1" && choice != "2" {
		fmt.Println("Do you want to register or login?")
		fmt.Println("1) Register")
		fmt.Println("2) Login")
		fmt.Print("Enter your choice (1 or 2): ")
		choice, _ = reader.ReadString('\n')
		choice = strings.TrimSpace(choice)
	}
	if choice == "1" {
	registration:
		username, password, computername, err = register(client)
		secondChoice := ""
		for err != nil && secondChoice != "y" && secondChoice != "n" {
			fmt.Print("\nRegistration failed, do you want to try again? (y/n): ")
			secondChoice, _ = reader.ReadString('\n')
			secondChoice = strings.TrimSpace(secondChoice)
		}
		if secondChoice == "n" {
			log.Print("Unable to register new user")
			return
		}
		if secondChoice == "y" {
			goto registration
		}
		log.Println("Registration successful")
	}
	if choice == "2" {
		username, password, computername = getLoginAndPassword()
	}
	log.Println("Trying to login...")
	token, err = client.Login(username, password, computername)
	if err != nil {
		log.Print("Unable to login")
		return
	}
	db.SetCfgValue("username", username)
	db.SetCfgValue("authencity_token", token)
	db.SetCfgValue("computer_name", computername)
	log.Print("Login successful!")
	return
}

func warningClearDataFolder(dataFolder string) {
	fmt.Printf("There was an unrecovarable error during application startup. Please manually remove the application folder %s", dataFolder)
}

var confPath string = ""

func Start() {
	log.Println("Starting cloudsyncer client.")
	confPath = *flag.String("cfgdir", "", "a string")
	if !toolkit.IsDirectory(confPath) {
		confPath = ""
	}
	log.Printf("Config path set to %s", confPath)
	log.Printf("Opening database file %s", getDbFilePath())
	if !toolkit.Exists(getDbFilePath()) {
		log.Println("Configuration database does not exist. Need to create one.")
		if prepareDatabase() != nil {
			log.Fatal("Oops, something went wrong. Exiting!")
		}
	}
	// _ := *flag.Bool("reset", false, "removes all data from files table, sets cursor to 0")
	appConfig["websocket"] = *flag.Bool("ws", false, "Sets client transmition to WebSocket")
	if toolkit.IsDirectory(getDbFilePath()) {
		log.Println("Error - database path should be a file, is a directory")
		warningClearDataFolder(getConfigFileDir())
		os.Exit(1)
	}
	err := db.InitDb(getDbFilePath(), logger)
	if err != nil {
		warningClearDataFolder(getConfigFileDir())
		os.Exit(1)
	}

	appConfig["work_dir"] = db.GetCfgValue("work_dir")
	if appConfig["work_dir"] == "" || !toolkit.IsDirectory(appConfig["work_dir"]) {
		appConfig["work_dir"], err = setupWorkDir()
		if err != nil {
			log.Println("Something went wrong: ", err)
			os.Exit(1)
		}
		db.SetCfgValue("work_dir", appConfig["work_dir"])
	}
	operations := make(chan FileOperation, 100)
	deltas := make(chan Delta, 100)
	client := NewClient(appConfig["work_dir"])
	listener := NewListener(deltas, client)
	worker := NewWorker(operations, deltas, client, listener, appConfig["work_dir"])
	watcher := NewWatcher(appConfig["work_dir"], operations, worker)

	err = initialConfig(client)
	if err != nil {
		log.Println("Something went wrong, ", err)
		os.Exit(1)
	}
	client.SetCredentials(appConfig["authencity_token"], appConfig["username"])
	os.MkdirAll(getTmpDir(), 0777)
	if err = worker.InitDb(); err != nil {
		log.Printf("worker error initializing database: %s", err)
		os.Exit(1)
	}
	wg := new(sync.WaitGroup)
	worker.Work()
	watcher.AddExcludedFolder(getTmpDir())
	if err = watcher.Init(); err != nil {
		log.Printf("Error initializing watcher: %s", err)
		os.Exit(1)
	}
	if err = worker.Sync(); err != nil {
		log.Printf("Worker error during syncing: %s", err)
		os.Exit(1)
	}
	if err != nil {
		log.Print("Error starting watcher: ", err)
	}
	watcher.Watch(wg)
	cursor := db.GetCfgValue("cursor")
	if cursor == "" {
		cursor = "0"
	}
	if err != nil {
		log.Fatal("failed to get cursor ", err)
	}
	if appConfig["websocket"] == true {
		listener.ListenWS(cursor)
	} else {
		listener.Listen(cursor)
	}
	log.Printf("Cloudsyncer started.")
	wg.Wait()
	log.Printf("Quitting cloudsyncer... have a nice day!")
}
