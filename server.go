package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/google/go-github/github"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var dbSession *mgo.Session
var users *mgo.Collection
var repos *mgo.Collection

var specialBranch string
var specialRef string

// Repo is a github repo
type Repo struct {
	ID               string `bson:"_id,omitempty"`
	Username         string
	AccessToken      string
	LastBuildOutput  string
	LastBuildSuccess bool
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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		payload := validate(r)
		if payload == nil {
			return
		}

		fullname := *payload.Repo.FullName
		url := *payload.Repo.URL

		repo := getRepo(fullname)
		if repo == nil {
			log.Printf("repo not known %s", fullname)
			return
		}

		path := "/tmp/" + fullname
		defer cleanUp(path)

		_, err := Clone(url, path, config.SpecialBranch)
		if err != nil {
			log.Printf("Could not clone: %s", err)
			return
		}

		HugoBuild(path)

		subrepo, err := Checkout(path + "/public/") // trailing / important
		if err != nil {
			log.Printf("Could not checkout: %s", err)
			return
		}

		err = Push(formatPushURL(repo.AccessToken, repo.Username, fullname), subrepo)
		if err != nil {
			log.Printf("Could not push: %s", err)
			return
		}
	})

	log.Println("Listening")
	log.Fatal(http.ListenAndServe(config.Address, nil))
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
