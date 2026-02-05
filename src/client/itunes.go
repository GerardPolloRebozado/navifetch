package client

import (
	"sync"

	. "github.com/torabit/itunes"
)

var lock = &sync.Mutex{}
var client *Client

func GetItunesClient() *Client {
	if client == nil {
		lock.Lock()
		defer lock.Unlock()
		if client == nil {
			client = New()
		}

	}
	return client
}
