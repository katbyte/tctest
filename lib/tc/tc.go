// Package tc is a minimal TeamCity REST API client for triggering builds and fetching build status and results.
package tc

import (
	"github.com/katbyte/tctest/lib/clog"
)

const TeamCityAPIVersion = "2026.1"

type Server struct {
	Server string
	token  *string
	User   *string
	Pass   *string
}

func NewServer(server, token, username, password string) Server {
	if token != "" {
		return NewServerUsingTokenAuth(server, token)
	}

	if username != "" {
		return NewServerUsingBasicAuth(server, username, password)
	}

	// should probably do something better here
	panic("token & username are both empty")
}

func NewServerUsingTokenAuth(server, token string) Server {
	clog.Log.Debugf("new tc: %s@%s", maskToken(token), server)
	return Server{
		Server: server,
		token:  &token,
	}
}

func NewServerUsingBasicAuth(server, username, password string) Server {
	clog.Log.Debugf("new tc: %s@%s", username, server)
	return Server{
		Server: server,
		User:   &username,
		Pass:   &password,
	}
}

func maskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return token[:4] + "****"
}
