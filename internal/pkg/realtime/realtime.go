package realtime

import (
	"github.com/googollee/go-socket.io"
	"github.com/jeogram/messenger/internal/pkg/ws"
)

// Broadcaster pushes realtime events to specific users over any transport.
type Broadcaster interface {
	SendToUsers(userIDs []string, payload ws.Outbound)
	OnlineUserIDs() []string
}

// WSBroadcaster delivers events via the raw WebSocket hub (/ws).
type WSBroadcaster struct{ Hub *ws.Hub }

func (b *WSBroadcaster) SendToUsers(userIDs []string, payload ws.Outbound) {
	b.Hub.SendToUsers(userIDs, payload)
}

func (b *WSBroadcaster) OnlineUserIDs() []string { return b.Hub.OnlineUserIDs() }

// SocketIOBroadcaster delivers events via the Socket.IO server.
// Each connected user joins a private room named "u:<userID>".
type SocketIOBroadcaster struct{ Server *socketio.Server }

// SocketIORoom returns the private room name for a user.
func SocketIORoom(userID string) string { return "u:" + userID }

func (b *SocketIOBroadcaster) SendToUsers(userIDs []string, payload ws.Outbound) {
	for _, uid := range userIDs {
		b.Server.BroadcastToRoom("/", SocketIORoom(uid), payload.Type, payload.Payload)
	}
}

func (b *SocketIOBroadcaster) OnlineUserIDs() []string { return nil }

// MultiBroadcaster fans a single event out to every transport.
type MultiBroadcaster struct{ Broadcasters []Broadcaster }

func (b *MultiBroadcaster) SendToUsers(userIDs []string, payload ws.Outbound) {
	for _, bc := range b.Broadcasters {
		bc.SendToUsers(userIDs, payload)
	}
}

func (b *MultiBroadcaster) OnlineUserIDs() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, bc := range b.Broadcasters {
		for _, id := range bc.OnlineUserIDs() {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}
