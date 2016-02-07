package main

import (
	"fmt"
	"html"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
		Clone("https://github.com/spencerlyon2/hugo_gh_blog.git", "/tmp/example-blog-2")
		HugoBuild("/tmp/example-blog-2")
		//Checkout(repo)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
