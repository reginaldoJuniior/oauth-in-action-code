package model

type Client struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client"`
	RedirectURIs []string `json:"redirect_uris"`
	Scope        string   `json:"scope"`
}
