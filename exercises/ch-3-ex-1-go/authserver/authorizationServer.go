package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
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

var codes = map[string]struct {
	ClientID    string
	Scope       []string
	User        string
	RedirectURI string
	State       string
}{}

var tokens = map[string]struct {
	ClientID string
	Scope    []string
	User     string
}{}

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

func randomString(n int) string {
	rand.Seed(time.Now().UnixNano())
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("authserver/files/index.html")
		if err != nil {
			http.Error(w, "Template not found", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, clients)
	})

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		clientID := r.URL.Query().Get("client_id")
		redirectURI := r.URL.Query().Get("redirect_uri")
		scope := r.URL.Query().Get("scope")
		state := r.URL.Query().Get("state")
		responseType := r.URL.Query().Get("response_type")

		client := getClient(clientID)
		if client == nil {
			tmpl, _ := template.ParseFiles("authserver/files/error.html")
			tmpl.Execute(w, map[string]string{"Error": "Unknown client"})
			return
		}
		validRedirect := false
		for _, uri := range client.RedirectURIs {
			if uri == redirectURI {
				validRedirect = true
				break
			}
		}
		if !validRedirect {
			tmpl, _ := template.ParseFiles("authserver/files/error.html")
			tmpl.Execute(w, map[string]string{"Error": "Invalid redirect URI"})
			return
		}
		requestedScope := strings.Fields(scope)
		allowedScope := strings.Fields(client.Scope)
		for _, s := range requestedScope {
			found := false
			for _, as := range allowedScope {
				if s == as {
					found = true
					break
				}
			}
			if !found {
				// invalid scope
				http.Redirect(w, r, redirectURI+"?error=invalid_scope", http.StatusFound)
				return
			}
		}
		if responseType != "code" {
			http.Redirect(w, r, redirectURI+"?error=unsupported_response_type", http.StatusFound)
			return
		}
		reqid := randomString(8)
		codes[reqid] = struct {
			ClientID    string
			Scope       []string
			User        string
			RedirectURI string
			State       string
		}{
			ClientID:    clientID,
			Scope:       requestedScope,
			User:        "",
			RedirectURI: redirectURI,
			State:       state,
		}
		tmpl, _ := template.ParseFiles("authserver/files/approve.html")
		err := tmpl.Execute(w, map[string]interface{}{"Client": client, "ReqID": reqid, "Scope": requestedScope})
		if err != nil {
			fmt.Print(err)
			return
		}
	})

	mux.HandleFunc("/approve", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return
		}
		reqid := r.Form.Get("reqid")
		approve := r.Form.Get("approve")
		user := r.Form.Get("user")
		codeData, ok := codes[reqid]
		if !ok {
			tmpl, _ := template.ParseFiles("authserver/files/error.html")
			err := tmpl.Execute(w, map[string]string{"Error": "No matching authorization request"})
			if err != nil {
				return
			}
			return
		}
		delete(codes, reqid)
		if approve != "" {
			code := randomString(8)
			codes[code] = struct {
				ClientID    string
				Scope       []string
				User        string
				RedirectURI string
				State       string
			}{
				ClientID:    codeData.ClientID,
				Scope:       codeData.Scope,
				User:        user,
				RedirectURI: codeData.RedirectURI,
				State:       codeData.State,
			}
			redirect := fmt.Sprintf("%s?code=%s&state=%s", codeData.RedirectURI, code, codeData.State)
			http.Redirect(w, r, redirect, http.StatusFound)
			return
		} else {
			redirect := fmt.Sprintf("%s?error=access_denied", codeData.RedirectURI)
			http.Redirect(w, r, redirect, http.StatusFound)
			return
		}
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return
		}
		clientID, clientSecret := "", ""
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Basic ") {
			decoded, _ := decodeBasicAuth(auth)
			clientID = decoded[0]
			clientSecret = decoded[1]
		}
		if r.Form.Get("client_id") != "" {
			if clientID != "" {
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
				return
			}
			clientID = r.Form.Get("client_id")
			clientSecret = r.Form.Get("client_secret")
		}
		client := getClient(clientID)
		if client == nil || client.ClientSecret != clientSecret {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
			return
		}
		code := r.Form.Get("code")
		codeData, ok := codes[code]
		if !ok || codeData.ClientID != clientID {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		accessToken := randomString(16)
		tokens[accessToken] = struct {
			ClientID string
			Scope    []string
			User     string
		}{
			ClientID: clientID,
			Scope:    codeData.Scope,
			User:     codeData.User,
		}
		delete(codes, code)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": accessToken,
			"token_type":   "Bearer",
			"scope":        strings.Join(codeData.Scope, " "),
		})
	})

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("files/authorizationServer"))))

	port := os.Getenv("AUTH_PORT")
	if port == "" {
		port = "9101"
	}
	fmt.Printf("OAuth Authorization Server is listening at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func decodeBasicAuth(auth string) ([]string, error) {
	payload := strings.TrimPrefix(auth, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	return parts, nil
}
