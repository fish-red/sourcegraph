package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"

	"sourcegraph.com/sourcegraph/go-selenium"
)

func init() {
	Register(&Test{
		Name:        "channel_flow",
		Description: "Creates a new channel and navigates to two pages via a websocket connection and 2 POST requests",
		Func:        testChannelFlow,
	})
}

func testChannelFlow(t *T) error {
	wd := t.WebDriver

	err := wd.Get(t.Endpoint("/-/channel/e2etest"))
	if err != nil {
		return err
	}

	// establish channel initialization page
	t.WaitForElement(selenium.ByXPATH, "//*[contains(text(), 'Click on a symbol in your editor to get started!')]")
	// check that the "connected" status appears
	t.WaitForElement(selenium.ByXPATH, "//*[contains(text(), 'connected')]")

	type Action struct {
		Repo    string `json:"Repo,omitempty"`
		Package string `json:"Package,omitempty"`
		Def     string `json:"Def,omitempty"`
		Error   string `json:"Error,omitempty"`
	}

	type Request struct {
		Action            Action `json:"Action,omitempty"`
		CheckForListeners bool   `json:"CheckForListeners,omitempty"`
	}

	// Test that the page changes to the gorilla/mux repo tree view after POST request
	u := &Request{Action: Action{
		Repo:    "github.com/gorilla/mux",
		Package: "github.com/gorilla/mux",
	}, CheckForListeners: true}
	body := new(bytes.Buffer)
	err = json.NewEncoder(body).Encode(u)
	if err != nil {
		return err
	}

	_, err = http.Post("https://grpc.sourcegraph.com/.api/channel/e2etest", "application/json; charset=utf-8", body)
	if err != nil {
		return err
	}

	t.WaitForRedirect("https://sourcegraph.com/github.com/gorilla/mux?utm_source=sourcegrapheditor", "wait for redirect to gorilla/mux repo")
	t.WaitForElement(selenium.ByXPATH, "//*[contains(text(), 'connected')]")

	// Test that the page changes to the definfo page of http.Post after POST request
	u = &Request{Action: Action{
		Repo:    "net/http",
		Package: "net/http",
		Def:     "Post",
	}, CheckForListeners: true}
	body = new(bytes.Buffer)
	err = json.NewEncoder(body).Encode(u)
	if err != nil {
		return err
	}

	_, err = http.Post("https://grpc.sourcegraph.com/.api/channel/e2etest", "application/json; charset=utf-8", body)
	if err != nil {
		return err
	}

	t.WaitForRedirect("https://sourcegraph.com/github.com/golang/go/-/info/GoPackage/net/http/-/Post?utm_source=sourcegrapheditor", "wait for redirect to homepage after sign-in")
	t.WaitForElement(selenium.ByXPATH, "//*[contains(text(), 'connected')]")

	return nil
}