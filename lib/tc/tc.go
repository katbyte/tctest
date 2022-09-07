package tc

import (
	"github.com/katbyte/tctest/lib/clog"
)

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
	clog.Log.Debugf("new tc: %s@%s", token, server)
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
