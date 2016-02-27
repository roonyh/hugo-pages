package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/google/go-github/github"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var dbSession *mgo.Session
var users *mgo.Collection
var repos *mgo.Collection

var specialBranch string
var specialRef string

var handlers map[int]*comm
var handlersMtx *sync.RWMutex

// Repo is a github repo
type Repo struct {
	ID              string `bson:"_id,omitempty"`
	Username        string
	AccessToken     string
	LastBuildOutput string
	LastBuildStatus string
}

// Comm is used to communicate to a worker goroutine
// stop is sent 'true' to ask the goroutine to stop
// stopped is sent 'true' when it stops
type comm struct {
	Stop    chan bool
	Stopped chan bool
}

func main() {
	config := loadConfig()
	config.print()

	specialBranch = config.SpecialBranch
	specialRef = "refs/heads/" + config.SpecialBranch

	dbSession, err := mgo.Dial(config.MongoURL)
	if err != nil {
		panic(err)
	}
	defer dbSession.Close()

	users = dbSession.DB("hugo-pages").C("users")
	repos = dbSession.DB("hugo-pages").C("repos")

	handlers = make(map[int]*comm)
	handlersMtx = &sync.RWMutex{}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		payload := validate(r)
		if payload == nil {
			return
		}

		fullname := *payload.Repo.FullName
		id := *payload.Repo.ID
		url := *payload.Repo.URL

		go func() {
			log.Printf("Got: %s", fullname)
			workerComm := synchronize(id)
			w := newWorker(id, url, workerComm)
			w.work()
			change := bson.M{
				"$set": bson.M{
					"lastbuildoutput": w.log,
					"lastbuildstatus": w.status,
				},
			}
			repos.UpdateId(fullname, change)
		}()
	})

	log.Println("Listening")
	log.Fatal(http.ListenAndServe(config.Address, nil))
}

// synchronize is a step to see if another goroutine is handling a request
// from the same repo
func synchronize(id int) *comm {
	handlersMtx.RLock()
	currentComm := handlers[id]
	handlersMtx.RUnlock()

	if currentComm == nil {
		// There is no goroutine handling a request for this repo
		handlersMtx.Lock()
		currentComm = handlers[id]
		if currentComm == nil {
			// Still there is no goroutine handling a request for this repo
			currentComm = &comm{
				Stop:    make(chan bool, 3), // Can buffer stop signals upto 3
				Stopped: make(chan bool, 1),
			}
			handlers[id] = currentComm
			handlersMtx.Unlock()
		} else {
			// Some goroutine has entered while we were creating a new channel
			handlersMtx.Unlock()
			_waitAndStart(currentComm, id)
		}
	} else {
		_waitAndStart(currentComm, id)
	}
	return currentComm
}

func _waitAndStart(currentComm *comm, id int) {
	currentComm.Stop <- true
	val := <-currentComm.Stopped
	if val == true {
		// This means previous goroutine does not know it was interrupted
		// So clear the signal it was sent
		<-currentComm.Stop
		currentComm.Stopped <- true
		//and resynchronize
		synchronize(id)
	}
}

type worker struct {
	stop    chan bool
	stopped chan bool
	id      int
	url     string
	path    string
	log     string
	status  string
}

func newWorker(id int, url string, workerComm *comm) *worker {
	return &worker{
		stop:    workerComm.Stop,
		stopped: workerComm.Stopped,
		id:      id,
		path:    fmt.Sprintf("/tmp/%d", id),
		url:     url,
		log:     "",
		status:  "incomplete",
	}
}

func (w *worker) work() {
	if !w.checkAndContinue("Starting build") {
		return
	}

	repo := getRepo(w.id)
	if repo == nil {
		w.checkAndStop("Repo not known")
		return
	}

	_, err := Clone(w.url, w.path, specialBranch)
	if err != nil {
		w.checkAndStop("Could not clone")
		return
	}

	if !w.checkAndContinue("building") {
		return
	}

	out, err := HugoBuild(w.path)
	if err != nil {
		cleanUp(w.path)
		w.checkAndStop("Could not build")
		return
	}
	log.Printf(out)

	if !w.checkAndContinue(out) {
		return
	}

	subrepo, err := Checkout(w.path + "/public/") // trailing / important
	if err != nil {
		cleanUp(w.path)
		w.checkAndStop("Could not checkout: " + err.Error())
		return
	}

	if !w.checkAndContinue("pushing") {
		return
	}

	err = Push(formatPushURL(repo.AccessToken, repo.Username, w.url), subrepo)
	if err != nil {
		cleanUp(w.path)
		log.Printf("Could not push: %s", err)
		return
	}
	cleanUp(w.path)
	w.checkAndStop("Finishing up")
	w.status = "complete"
}

func (w *worker) checkAndContinue(msg string) bool {
	select {
	case <-w.stop:
		log.Printf("Stopping: %s %s", msg, w.url)
		w.log = w.log + "Stopping: " + msg + "\n"
		cleanUp(w.path)
		w.stopped <- false
		return false
	default:
		log.Printf("Continuing %s", msg)
		w.log = w.log + msg + "\n"
		return true
	}
}

func (w *worker) checkAndStop(msg string) {
	select {
	case <-w.stop:
		log.Printf("Stopping: %s %s", msg, w.url)
		w.log = w.log + "Stopping: " + msg + "\n"
		cleanUp(w.path)
		w.stopped <- false
	default:
		handlersMtx.Lock()
		delete(handlers, w.id)
		handlersMtx.Unlock()
		log.Printf("Stopping (completed): %s", msg)
		w.log = w.log + "Stopping (completed): " + msg + "\n"
		cleanUp(w.path)
		w.stopped <- true
	}
}

func validate(r *http.Request) *github.WebHookPayload {
	if r.Method != "POST" {
		log.Println("Not POST")
		return nil
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil
	}

	var t github.WebHookPayload
	err = json.Unmarshal(b, &t)
	if err != nil {
		return nil
	}

	if t.Ref == nil {
		return nil
	}

	if *t.Ref != specialRef {
		log.Println("Not hugo pages:", *t.Ref)
		return nil
	}

	mac := hmac.New(sha1.New, []byte("hugopagessecret"))
	mac.Write(b)
	if err != nil {
		log.Println(err)
		return nil
	}

	expectedMAC := mac.Sum(nil)
	signature := r.Header["X-Hub-Signature"][0]
	expected := "sha1=" + hex.EncodeToString(expectedMAC)

	if !hmac.Equal([]byte(signature), []byte(expected)) {
		log.Println("unknown origin")
		return nil
	}

	return &t
}

func formatPushURL(accessToken, username, url string) string {
	// TODO: optimize string concat
	parts := strings.Split(url, "://")
	return parts[0] + "://" + username + ":" + accessToken + parts[1]
}

func getRepo(id int) *Repo {
	result := &Repo{}
	err := repos.FindId(id).One(result)
	if err != nil {
		log.Println(err)
		return nil
	}

	return result
}

func cleanUp(path string) {
	err := os.RemoveAll(path)
	if err != nil {
		log.Println(err)
	}
}
