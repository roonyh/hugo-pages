package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
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

var handlers map[string]*comm
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

	handlers = make(map[string]*comm)
	handlersMtx = &sync.RWMutex{}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		payload := validate(r)
		if payload == nil {
			return
		}

		fullname := *payload.Repo.FullName
		url := *payload.Repo.URL

		go func() {
			log.Printf("Got: %s", fullname)
			workerComm := synchronize(fullname)
			w := newWorker(fullname, url, workerComm)
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
func synchronize(fullname string) *comm {
	// TODO: Make sure that the goroutine that overtakes is actually a new one
	handlersMtx.RLock()
	currentComm := handlers[fullname]
	handlersMtx.RUnlock()

	if currentComm == nil {
		// There is no goroutine handling a request for this repo
		handlersMtx.Lock()
		currentComm = handlers[fullname]
		if currentComm == nil {
			// Still there is no goroutine handling a request for this repo
			currentComm = &comm{
				Stop:    make(chan bool, 3), // Can buffer stop signals upto 3
				Stopped: make(chan bool, 1),
			}
			handlers[fullname] = currentComm
			handlersMtx.Unlock()
		} else {
			// Some goroutine has entered while we were creating a new channel
			handlersMtx.Unlock()
			_waitAndStart(currentComm, fullname)
		}
	} else {
		_waitAndStart(currentComm, fullname)
	}
	return currentComm
}

func _waitAndStart(currentComm *comm, fullname string) {
	currentComm.Stop <- true
	val := <-currentComm.Stopped
	if val == true {
		// This means previous goroutine does not know it was interrupted
		// So clear the signal it was sent
		<-currentComm.Stop
		currentComm.Stopped <- true
		//and resynchronize
		synchronize(fullname)
	}
}

type worker struct {
	stop     chan bool
	stopped  chan bool
	fullname string
	url      string
	log      string
	status   string
}

func newWorker(fullname, url string, workerComm *comm) *worker {
	return &worker{
		stop:     workerComm.Stop,
		stopped:  workerComm.Stopped,
		fullname: fullname,
		url:      url,
		log:      "",
		status:   "incomplete",
	}
}

func (w *worker) work() {
	w.checkAndContinue("starting processing")

	repo := getRepo(w.fullname)
	if repo == nil {
		w.checkAndStop("repo not known")
		return
	}

	path := "/tmp/" + w.fullname
	defer cleanUp(path)

	_, err := Clone(w.url, path, specialBranch)
	if err != nil {
		w.checkAndStop("could not clone")
		return
	}

	w.checkAndContinue("building")
	out, err := HugoBuild(path)
	if err != nil {
		cleanUp(path)
		w.checkAndStop("could not build")
		return
	}
	log.Printf(out)
	w.checkAndContinue(out)

	subrepo, err := Checkout(path + "/public/") // trailing / important
	if err != nil {
		cleanUp(path)
		w.checkAndStop("Could not checkout: " + err.Error())
		return
	}

	w.checkAndContinue("pushing")

	err = Push(formatPushURL(repo.AccessToken, repo.Username, w.fullname), subrepo)
	if err != nil {
		cleanUp(path)
		log.Printf("Could not push: %s", err)
		return
	}
	cleanUp(path)
	w.checkAndStop("finishing up")
	w.status = "complete"
}

func (w *worker) checkAndContinue(msg string) {
	select {
	case <-w.stop:
		log.Printf("stopping: %s %s", msg, w.fullname)
		w.log = w.log + "stopping: " + msg + "\n"
		w.stopped <- false
	default:
		log.Printf("continuing %s %s", msg, w.fullname)
		w.log = w.log + "continuing: " + msg + "\n"
	}
}

func (w *worker) checkAndStop(msg string) {
	select {
	case <-w.stop:
		log.Printf("stopping: %s %s", msg, w.fullname)
		w.log = w.log + "stopping: " + msg + "\n"
		w.stopped <- false
	default:
		handlersMtx.Lock()
		delete(handlers, w.fullname)
		handlersMtx.Unlock()
		log.Printf("stopping (completed): %s %s", msg, w.fullname)
		w.log = w.log + "stopping (completed): " + msg + "\n"
		w.stopped <- true
	}
}

func validate(r *http.Request) *github.WebHookPayload {
	if r.Method != "POST" {
		log.Println("Not POST")
		return nil
	}

	decoder := json.NewDecoder(r.Body)
	var t github.WebHookPayload
	err := decoder.Decode(&t)
	if err != nil {
		return nil
	}

	if *t.Ref != specialRef {
		log.Println("Not hugo pages:", *t.Ref)
		return nil
	}

	return &t
}

func formatPushURL(accessToken, username, fullname string) string {
	// TODO: optimize string concat
	return "https://" + username + ":" + accessToken + "@github.com/" + fullname
}

func getRepo(fullname string) *Repo {
	result := &Repo{}
	err := repos.Find(bson.M{"_id": fullname}).One(result)
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
