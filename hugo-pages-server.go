package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/google/go-github/github"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		payload := validate(r)
		if payload == nil {
			return
		}
		fmt.Println(*payload.Repo.URL)
		fmt.Println("/tmp/", *payload.Repo.FullName)
		url := *payload.Repo.FullName
		path := "/tmp/" + *payload.Repo.FullName
		_, err := Clone(*payload.Repo.URL, path, "hg-pages")
		fmt.Println(err)

		if err != nil {
			return
		}

		HugoBuild(path)
		subrepo := Checkout(path + "/public/") // trailing / important
		Push(url, subrepo)
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
