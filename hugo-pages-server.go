package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-github/github"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var dbSession *mgo.Session
var users *mgo.Collection

// Repo is a github repo
type Repo struct {
	ID          string `bson:"_id,omitempty"`
	Username    string
	AccessToken string
}

func main() {
	dbSession, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	defer dbSession.Close()

	users = dbSession.DB("ghpages").C("users")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		payload := validate(r)
		if payload == nil {
			return
		}
		fmt.Println(*payload.Repo.URL)
		fullname := *payload.Repo.FullName
		fmt.Println("/tmp/", *payload.Repo.FullName)
		url := *payload.Repo.URL
		path := "/tmp/" + fullname
		_, err := Clone(url, path, "hg-pages")
		fmt.Println(err)

		if err != nil {
			return
		}

		repo := getRepo(fullname)
		HugoBuild(path)
		subrepo := Checkout(path + "/public/") // trailing / important
		Push(formatPushURL(repo.AccessToken, repo.Username, fullname), subrepo)
	})

	fmt.Println("listening")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func validate(r *http.Request) *github.WebHookPayload {
	fmt.Println("valling")
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

	if *t.Ref != "refs/heads/hg-pages" {
		log.Println("Not hg-pages:", *t.Ref)
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
	err := users.Find(bson.M{"_id": fullname}).One(result)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return result
}
