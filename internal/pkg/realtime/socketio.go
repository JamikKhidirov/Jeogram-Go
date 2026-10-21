package realtime

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/googollee/go-socket.io"
	"github.com/googollee/go-socket.io/engineio"
	"github.com/rs/zerolog/log"
)

// ErrSocketIOUnauthorized is returned when a Socket.IO handshake carries no
// valid access token.
var ErrSocketIOUnauthorized = errors.New("unauthorized")

// SocketIOServer delivers the same realtime events as the raw WebSocket hub
// (/ws) over the Socket.IO protocol (v2, Engine.IO v3).
//
// Every authenticated client joins a private room "u:<userID>", so the existing
// SocketIOBroadcaster can push events without any additional plumbing.
//
// Handshake URL:
//
//	ws://host/socket.io/?token=<access_token>&EIO=3&transport=websocket
type SocketIOServer struct {
	Server *socketio.Server
}

// NewSocketIOServer creates a Socket.IO server. The authenticate callback gets
// the JWT taken from the `token` (or `access_token`) query parameter and must
// return the user ID.
func NewSocketIOServer(authenticate func(token string) (string, error)) *SocketIOServer {
	sio := socketio.NewServer(&engineio.Options{})
	s := &SocketIOServer{Server: sio}

	sio.OnConnect("/", func(conn socketio.Conn) error {
		u := conn.URL()
		userID := u.Query().Get("user_id")
		if userID == "" {
			return ErrSocketIOUnauthorized
		}
		sio.JoinRoom("/", SocketIORoom(userID), conn)
		log.Debug().Str("user", userID).Msg("socket.io client connected")
		return nil
	})
	sio.OnDisconnect("/", func(conn socketio.Conn, reason string) {
		u := conn.URL()
		log.Debug().Str("user", u.Query().Get("user_id")).Str("reason", reason).Msg("socket.io client disconnected")
	})

	return s
}

// ServeHTTP authenticates the handshake and proxies it to the Socket.IO server.
// The resolved user ID is injected into the query so that the connect handler
// can join the private room.
func (s *SocketIOServer) ServeHTTP(w http.ResponseWriter, r *http.Request, authenticate func(token string) (string, error)) {
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.URL.Query().Get("access_token")
	}
	if token == "" {
		http.Error(w, "missing authorization token", http.StatusUnauthorized)
		return
	}
	userID, err := authenticate(token)
	if err != nil {
		http.Error(w, "invalid or expired token", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()
	q.Set("user_id", userID)
	r2 := r.Clone(r.Context())
	r2.URL = &url.URL{
		Scheme:   r.URL.Scheme,
		Host:     r.URL.Host,
		Path:     r.URL.Path,
		RawQuery: q.Encode(),
	}
	s.Server.ServeHTTP(w, r2)
}
