package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

const page = `<!DOCTYPE html>
<html><head><title>Hello</title></head>
<body><h1>Hello, World!</h1></body></html>
`

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, page)
	})
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
