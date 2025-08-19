package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
)

var resource = struct {
	Name        string
	Description string
}{
	Name:        "Protected Resource",
	Description: "This data has been protected by OAuth 2.0",
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("files/protectedResource/index.html")
		if err != nil {
			http.Error(w, "Template not found", http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, resource)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("files/protectedResource"))))

	port := os.Getenv("RESOURCE_PORT")
	if port == "" {
		port = "9002"
	}
	fmt.Printf("OAuth Resource Server is listening at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
