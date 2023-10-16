/*
This is an example application to demonstrate querying the user info endpoint.
*/
package main

import (
	"azuread-play/internal/azureclient"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func setCallbackCookie(w http.ResponseWriter, r *http.Request, name, value string) {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   int(time.Hour.Seconds()),
		Secure:   r.TLS != nil,
		HttpOnly: true,
	}
	http.SetCookie(w, c)
}

const (
	port              = ":5556"
	domain            = "localhost"
	discoveryBaseURLT = "https://login.microsoftonline.com/%s/v2.0"
	issuerUrlT        = "https://login.microsoftonline.com/%s/v2.0"
)

var (
	fullDomain = domain + port
)

func main() {
	ctx := context.Background()
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	// create a logger and add it to the context
	log := zerolog.New(os.Stdout).With().Caller().Timestamp().Logger()
	ctx = log.WithContext(ctx)

	err := godotenv.Overload()
	if err != nil {
		log.Fatal().Err(err).Msg("Error loading .env file")
	}
	tenantID := os.Getenv("AZURE_TENANT_ID")
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")

	client, err := azureclient.NewClient(tenantID, clientID, clientSecret)
	if err != nil {
		log.Fatal().Err(err).Msg("Error creating client")
		client = nil
	}

	discoveryBaseURL := fmt.Sprintf(discoveryBaseURLT, tenantID)
	issuerUrl := fmt.Sprintf(issuerUrlT, tenantID)

	ctx = oidc.InsecureIssuerURLContext(ctx, issuerUrl)

	provider, err := oidc.NewProvider(ctx, discoveryBaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to query provider")
	}
	endpoint := provider.Endpoint()
	config := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     endpoint,
		RedirectURL:  "http://localhost:5556/auth/callback",
		Scopes: []string{
			oidc.ScopeOpenID,
			"profile",
			"email",
			"User.Read",
			//	"Group.Read",
		},
	}

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		state, err := randString(16)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		setCallbackCookie(w, r, "state", state)

		http.Redirect(w, r, config.AuthCodeURL(state), http.StatusFound)
	})

	http.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		state, err := r.Cookie("state")
		if err != nil {
			http.Error(w, "state not found", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("state") != state.Value {
			http.Error(w, "state did not match", http.StatusBadRequest)
			return
		}

		oauth2Token, err := config.Exchange(ctx, r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		userInfo, err := provider.UserInfo(ctx, oauth2.StaticTokenSource(oauth2Token))
		if err != nil {
			http.Error(w, "Failed to get userinfo: "+err.Error(), http.StatusInternalServerError)
			return
		}
		groups := []string{}
		if client == nil {
			// pull the groups directly from the MSGRAPH api
			user, err := client.GetUserByEmail(ctx, userInfo.Email)
			if err != nil {
				log.Error().Err(err).Msg("Error getting user")
			} else {
				client.IterateUserGroups(ctx, user.ID, func(group string) bool {
					groups = append(groups, group)
					return true
				})
			}
		}

		// decode access_token
		jwtIdToken := oauth2Token.Extra("id_token").(string)
		var idToken *jwt.Token
		jwtParser := jwt.NewParser()
		idToken, _, err = jwtParser.ParseUnverified(jwtIdToken, jwt.MapClaims{})
		if err != nil {
			log.Error().Err(err).Msg("Error parsing JWT")
		}
		accessToken, _, err := jwtParser.ParseUnverified(oauth2Token.AccessToken, jwt.MapClaims{})
		if err != nil {
			log.Error().Err(err).Msg("Error parsing JWT - accessToken")
		}
		resp := struct {
			UserInfo          *oidc.UserInfo
			Groups            []string
			IdTokenParsed     *jwt.Token
			AccessTokenParsed *jwt.Token
		}{userInfo, groups, idToken, accessToken}

		data, err := json.MarshalIndent(resp, "", "    ")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write(data)
	})

	log.Printf("listening on http://%s/", fullDomain)
	err = http.ListenAndServe(fullDomain, nil)
	if err != nil {
		log.Fatal().Err(err).Msg("Error listening")
	}

}
