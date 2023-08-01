package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

/*
	These commands are conveneience CLI tools that operate on a
	running GigaWallet by calling the admin REST API.
*/

// SetSyncHeight resets the sync height for GigaWallet, which will cause
// chain-follower to start re-scanning the blockchain from that point.
// This can be used when starting a NEW GigaWallet instance to avoid
// scanning the entire blockchain, if you have no imported wallets with
// old transactions.
//
// WARNING:  Using this on an active/production GigaWallet will PAUSE
// the discovery of any new transactions until the re-scan has completed,
// meaning that any users waiting for Invoice Paid confirmations will
// be on hold until everything is reindexed. USE WITH CAUTION.

func SetSyncHeight(blockHeight string, c giga.Config, s SubCommandArgs) error {
	fmt.Println(s)
	url, err := adminAPIURL(c, s, fmt.Sprintf("/admin/setsyncheight/%s", blockHeight))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Calling", url)
	return postURL(url, "")
}

// work out the remote admin URL from args or config and return
// a complete path with our best guess
func adminAPIURL(c giga.Config, s SubCommandArgs, path string) (string, error) {
	base := ""
	if s.RemoteAdminServer != "" {
		base = s.RemoteAdminServer
	} else {
		host := c.WebAPI.AdminBind
		if host == "" {
			host = "localhost"
		}
		base = fmt.Sprintf("http://%s:%s/", host, c.WebAPI.AdminPort)
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}

	p, err := url.Parse(path)
	if err != nil {
		return "", err
	}

	return u.ResolveReference(p).String(), nil
}

// post a command to a remote GigaWallet admin API
// XXX will probably get refactored, rather limited
func postURL(url string, body interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to serialize request body: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status code: %d", resp.StatusCode)
	}

	return nil
}
