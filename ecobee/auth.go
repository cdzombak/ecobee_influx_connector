package ecobee

// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// This file contains authentication related functions and structs.

// Scopes defines the scopes we request from the API.
var Scopes = []string{"smartRead", "smartWrite"}

type tokenSource struct {
	token               oauth2.Token
	cacheFile, clientID string
}

func TokenSource(clientID, cacheFile string) oauth2.TokenSource {
	return oauth2.ReuseTokenSource(nil, newTokenSource(clientID, cacheFile))
}

func newTokenSource(clientID, cacheFile string) *tokenSource {
	file, err := ioutil.ReadFile(cacheFile)
	if err != nil {
		// no file, corrupted, or other problem: just start with an
		// empty token.
		return &tokenSource{clientID: clientID, cacheFile: cacheFile}
	}
	var tok oauth2.Token
	err = json.Unmarshal(file, &tok)
	if err != nil {
		// can't unmarshal?  Return an empty token.
		return &tokenSource{clientID: clientID, cacheFile: cacheFile}
	}
	return &tokenSource{clientID: clientID, cacheFile: cacheFile, token: tok}
}

func (ts *tokenSource) save() error {
	d, err := json.Marshal(ts.token)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(ts.cacheFile, d, 0777)
	return err
}

type PinResponse struct {
	EcobeePin string `json:"ecobeePin"`
	Code      string `json:"code"`
}

// Interactive authentication, triggered on initial use of the client
func (ts *tokenSource) firstAuth() error {
	pinResponse, err := ts.authorize()
	if err != nil {
		return err
	}
	fmt.Printf("Pin is %q\nPress <enter> after authorizing it on https://www.ecobee.com/consumerportal in the menu"+
		" under 'My Apps'\n", pinResponse.EcobeePin)
	var input string
	fmt.Scanln(&input)
	return ts.accessToken(pinResponse.Code)
}

// Make a pin request to ecobee and return the pin and code
func (ts *tokenSource) authorize() (*PinResponse, error) {
	uv := url.Values{
		"response_type": {"ecobeePin"},
		"client_id":     {ts.clientID},
		"scope":         {strings.Join(Scopes, ",")},
	}
	u := url.URL{
		Scheme:   "https",
		Host:     "api.ecobee.com",
		Path:     "authorize",
		RawQuery: uv.Encode(),
	}

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("error retrieving response: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid server response: %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %s", err)
	}

	var r PinResponse
	err = json.Unmarshal(body, &r)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %s", err)
	}
	return &r, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // nonstandard
	TokenType    string `json:"token_type"`
}

func (tr *tokenResponse) Token() oauth2.Token {
	tok := oauth2.Token{
		AccessToken:  tr.AccessToken,
		TokenType:    tr.TokenType,
		RefreshToken: tr.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}
	return tok
}

func (ts *tokenSource) accessToken(code string) error {
	return ts.getToken(url.Values{
		"grant_type": {"ecobeePin"},
		"client_id":  {ts.clientID},
		"code":       {code},
	})
}
func (ts *tokenSource) refreshToken() error {
	return ts.getToken(url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {ts.clientID},
		"refresh_token": {ts.token.RefreshToken},
	})
}

func (ts *tokenSource) getToken(uv url.Values) error {
	u := url.URL{
		Scheme:   "https",
		Host:     "api.ecobee.com",
		Path:     "token",
		RawQuery: uv.Encode(),
	}
	resp, err := http.PostForm(u.String(), nil)
	if err != nil {
		return fmt.Errorf("error POSTing request: %s", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("invalid server response: %v", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %s", err)
	}

	var r tokenResponse
	err = json.Unmarshal(body, &r)
	if err != nil {
		return fmt.Errorf("error unmarshalling response: %s", err)
	}

	ts.token = r.Token()
	if !ts.token.Valid() {
		return fmt.Errorf("invalid token")
	}
	err = ts.save()
	if err != nil {
		return fmt.Errorf("error saving token: %s", err)
	}
	return nil
}

func (ts *tokenSource) Token() (*oauth2.Token, error) {
	if !ts.token.Valid() {
		if len(ts.token.RefreshToken) > 0 {
			err := ts.refreshToken()
			if err != nil {
				return nil, fmt.Errorf("error refreshing token: %s", err)
			}
		} else {
			err := ts.firstAuth()
			if err != nil {
				return nil, fmt.Errorf("error on initial authentication: %s", err)
			}
		}
	}
	return &ts.token, nil
}

// Client represents the Ecobee API client.
type Client struct {
	*http.Client
}

// NewClient creates a Ecobee API client for the specific clientID
// (Application Key).  Use the Ecobee Developer Portal to create the
// Application Key.
// (https://www.ecobee.com/consumerportal/index.html#/dev)
func NewClient(clientID, cacheFile string) *Client {
	return &Client{oauth2.NewClient(
		context.Background(), TokenSource(clientID, cacheFile))}
}

// Authorize retrieves an ecobee Pin and Code, allowing calling code to present them to the user
// outside of the ecobee request context.
// This is useful when non-interactive authorization is required.
// For example: an app being deployed and authorized using ansible, which does not support interacting with commands.
func Authorize(clientID string) (*PinResponse, error) {
	return newTokenSource(clientID, "").authorize()
}

// SaveToken retreives a new token from ecobee and saves it to the auth cache
// after a pin/code combination has been added by an ecobee user.
func SaveToken(clientID string, cacheFile string, code string) error {
	return newTokenSource(clientID, cacheFile).accessToken(code)
}
