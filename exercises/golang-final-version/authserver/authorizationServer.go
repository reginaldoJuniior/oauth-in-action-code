package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
)

// In-memory stores
var clients = []struct {
	ClientID     string
	ClientSecret string
	RedirectURIs []string
	Scope        string
}{
	{
		ClientID:     "oauth-client-1",
		ClientSecret: "oauth-client-secret-1",
		RedirectURIs: []string{"http://localhost:9100/callback"},
		Scope:        "foo bar",
	},
}

func getClient(clientID string) *struct {
	ClientID     string
	ClientSecret string
	RedirectURIs []string
	Scope        string
} {
	for _, c := range clients {
		if c.ClientID == clientID {
			return &c
		}
	}
	return nil
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/authorize", authorizeHandler)
	mux.HandleFunc("/approve", approveHandler)
	mux.HandleFunc("/token", tokenHandler)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("authserver/files/index.html")
		if err != nil {
			http.Error(w, "Template not found", http.StatusInternalServerError)
			return
		}
		err = tmpl.Execute(w, clients)
		if err != nil {
			fmt.Print(err.Error())
			return
		}
	})

	//mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("files/authorizationServer"))))

	port := os.Getenv("AUTH_PORT")
	if port == "" {
		port = "9101"
	}
	fmt.Printf("OAuth Authorization Server is listening at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
