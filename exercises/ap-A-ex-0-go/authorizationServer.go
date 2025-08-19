package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("files/authorizationServer/index.html")
		if err != nil {
			http.Error(w, "Template not found", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("files/authorizationServer"))))

	port := os.Getenv("AUTH_PORT")
	if port == "" {
		port = "9001"
	}
	fmt.Printf("OAuth Authorization Server is listening at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
