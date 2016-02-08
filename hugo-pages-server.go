package main

import (
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		Clone("https://github.com/roonyh/blog.git", "/tmp/example-blog-8", "hg-pages")
		HugoBuild("/tmp/example-blog-2")
		//Checkout(repo)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
