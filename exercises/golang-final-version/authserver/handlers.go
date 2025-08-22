package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

const (
	approvePageFile = "authserver/files/approve.html"
	errorPageFile   = "authserver/files/error.html"
)

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

func authorizeHandler(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	scope := r.URL.Query().Get("scope")
	state := r.URL.Query().Get("state")
	responseType := r.URL.Query().Get("response_type")

	client := getClient(clientID)
	if client == nil {
		tmpl, _ := template.ParseFiles(errorPageFile)
		err := tmpl.Execute(w, map[string]string{"Error": "Unknown client"})
		if err != nil {
			fmt.Print(err.Error())
			return
		}
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
		tmpl, _ := template.ParseFiles(errorPageFile)
		err := tmpl.Execute(w, map[string]string{"Error": "Invalid redirect URI"})
		if err != nil {
			fmt.Print(err.Error())
			return
		}
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

	reqID := randomString(8)
	codes[reqID] = struct {
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
	tmpl, _ := template.ParseFiles(approvePageFile)
	err := tmpl.Execute(w, map[string]interface{}{"Client": client, "ReqID": reqID, "Scope": requestedScope})
	if err != nil {
		fmt.Print(err)
		return
	}
}

func randomString(n int) string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]rune, n)
	charSet := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	for i := range b {
		b[i] = charSet[seededRand.Intn(len(charSet))]
	}
	return string(b)
}

func approveHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	reqID := r.Form.Get("reqID")
	approve := r.Form.Get("approve")
	user := r.Form.Get("user")
	codeData, ok := codes[reqID]
	if !ok {
		tmpl, _ := template.ParseFiles(errorPageFile)
		err := tmpl.Execute(w, map[string]string{"Error": "No matching authorization request"})
		if err != nil {
			return
		}
		return
	}
	delete(codes, reqID)
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
}

func tokenHandler(w http.ResponseWriter, r *http.Request) {
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
			err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
			if err != nil {
				fmt.Print(err)
				return
			}
			return
		}
		clientID = r.Form.Get("client_id")
		clientSecret = r.Form.Get("client_secret")
	}
	client := getClient(clientID)
	if client == nil || client.ClientSecret != clientSecret {
		w.WriteHeader(http.StatusUnauthorized)
		err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
		if err != nil {
			fmt.Print(err)
			return
		}
		return
	}
	code := r.Form.Get("code")
	codeData, ok := codes[code]
	if !ok || codeData.ClientID != clientID {
		w.WriteHeader(http.StatusUnauthorized)
		err := json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
		if err != nil {
			fmt.Print(err)
			return
		}
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
	err := json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"scope":        strings.Join(codeData.Scope, " "),
	})
	if err != nil {
		fmt.Print(err)
		return
	}
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
