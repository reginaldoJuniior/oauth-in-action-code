package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
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
		tmpl, err := template.ParseFiles("protected/files/index.html")
		if err != nil {
			http.Error(w, "Template not found", http.StatusInternalServerError)
			fmt.Println(err)
			return
		}
		err = tmpl.Execute(w, resource)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			fmt.Println("Error executing template:", err)
			return
		}
	})

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("protected/files/protectedResource"))))

	mux.HandleFunc("/resource", func(w http.ResponseWriter, r *http.Request) {
		var inToken string
		// Check Authorization header
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			inToken = auth[7:]
		} else if err := r.ParseForm(); err == nil {
			// Check form body
			inToken = r.Form.Get("access_token")
		}
		if len(inToken) > 0 { // Replace with real token validation
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(resource)
			if err != nil {
				fmt.Errorf("Error writing response: %v", err)
				return
			}
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	})

	port := os.Getenv("RESOURCE_PORT")
	if port == "" {
		port = "9002"
	}
	fmt.Printf("OAuth Resource Server is listening at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
