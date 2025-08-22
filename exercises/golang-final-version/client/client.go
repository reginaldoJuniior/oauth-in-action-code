package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var client = struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}{
	ClientID:     "oauth-client-1",
	ClientSecret: "oauth-client-secret-1",
	RedirectURI:  "http://localhost:9100/callback",
}

var authPort = os.Getenv("AUTH_PORT")

var authServer = struct {
	AuthorizationEndpoint string
	TokenEndpoint         string
}{
	AuthorizationEndpoint: "http://localhost:9101/authorize",
	TokenEndpoint:         "http://localhost:9101/token",
}

func loadAuthServerConfig() {
	if authPort == "" {
		authPort = "9001"
	}
	authServer.AuthorizationEndpoint = "http://localhost:" + authPort + "/authorize"
	authServer.TokenEndpoint = "http://localhost:" + authPort + "/token"
}

var accessToken string
var state string
var scope string

func randomString(n int) string {
	rand.Seed(time.Now().UnixNano())
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func encodeClientCredentials(id, secret string) string {
	return base64.StdEncoding.EncodeToString([]byte(id + ":" + secret))
}

func main() {
	loadAuthServerConfig()
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("client/files/index.html")
		if err != nil {
			http.Error(w, "Template not found", http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, map[string]interface{}{"access_token": accessToken, "scope": scope})
	})

	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		accessToken = ""
		state = randomString(8)
		v := url.Values{}
		v.Set("response_type", "code")
		v.Set("client_id", client.ClientID)
		v.Set("redirect_uri", client.RedirectURI)
		v.Set("state", state)
		v.Set("scope", "foo bar")
		redirectURL := authServer.AuthorizationEndpoint + "?" + v.Encode()
		http.Redirect(w, r, redirectURL, http.StatusFound)
	})

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("error") != "" {
			tmpl, _ := template.ParseFiles("client/files/error.html")
			tmpl.Execute(w, map[string]string{"Error": r.URL.Query().Get("error")})
			return
		}
		if r.URL.Query().Get("state") != state {
			tmpl, _ := template.ParseFiles("client/files/error.html")
			tmpl.Execute(w, map[string]string{"Error": "State value did not match"})
			return
		}
		code := r.URL.Query().Get("code")
		v := url.Values{}
		v.Set("grant_type", "authorization_code")
		v.Set("code", code)
		v.Set("redirect_uri", client.RedirectURI)
		req, _ := http.NewRequest("POST", authServer.TokenEndpoint, strings.NewReader(v.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Basic "+encodeClientCredentials(client.ClientID, client.ClientSecret))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			tmpl, _ := template.ParseFiles("client/files/error.html")
			tmpl.Execute(w, map[string]string{"Error": "Unable to fetch access token"})
			return
		}
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var tokenResp map[string]interface{}
			json.Unmarshal(body, &tokenResp)
			accessToken, _ = tokenResp["access_token"].(string)
			scope, _ = tokenResp["scope"].(string)
			tmpl, _ := template.ParseFiles("client/files/index.html")
			tmpl.Execute(w, map[string]interface{}{"access_token": accessToken, "scope": scope})
		} else {
			tmpl, _ := template.ParseFiles("client/files/error.html")
			tmpl.Execute(w, map[string]string{"Error": "Unable to fetch access token, server response: " + resp.Status})
		}
	})

	mux.HandleFunc("/fetch_resource", func(w http.ResponseWriter, r *http.Request) {
		if accessToken == "" {
			tmpl, _ := template.ParseFiles("client/files/error.html")
			err := tmpl.Execute(w, map[string]string{"Error": "Missing access token"})
			if err != nil {
				fmt.Println(err)
				return
			}
			return
		}
		req, _ := http.NewRequest("POST", "http://localhost:9102/resource", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			tmpl, _ := template.ParseFiles("client/files/error.html")
			err := tmpl.Execute(w, map[string]string{"Error": "Unable to fetch resource"})
			if err != nil {
				fmt.Println(err)
				return
			}
			return
		}
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var resourceData map[string]interface{}
			err := json.Unmarshal(body, &resourceData)
			if err != nil {
				fmt.Println(err)
				return
			}
			// Pretty-print the JSON string before passing to the template
			prettyJSON, err := json.MarshalIndent(resourceData, "", "  ")
			if err != nil {
				fmt.Println(err)
				return
			}
			tmpl, _ := template.ParseFiles("client/files/data.html")
			err = tmpl.Execute(w, map[string]interface{}{"resource": string(prettyJSON)})
			if err != nil {
				fmt.Println(err)
				return
			}
		} else {
			tmpl, _ := template.ParseFiles("client/files/error.html")
			err := tmpl.Execute(w, map[string]string{"Error": "Unable to fetch resource, server response: " + resp.Status})
			if err != nil {
				fmt.Println(err)
				return
			}
		}
	})

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("files/client"))))

	port := os.Getenv("CLIENT_PORT")
	if port == "" {
		port = "9100"
	}
	fmt.Printf("OAuth Client is listening at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
