package packets

// Client bound packets that the server sends to connected clients.

// ClientBoundConnected represents a successful connection acknowledgement.
type ClientBoundConnected struct{}

// ClientBoundDisconnect notifies the client of a disconnection and reason.
type ClientBoundDisconnect struct {
	Reason string `json:"r"`
}

// ClientBoundMessage delivers a chat message to the client.
type ClientBoundMessage struct {
	Sender  string `json:"s"`
	Message string `json:"m"`
}

// ClientBoundUpdateUserList synchronises the username -> in-game-name mapping.
type ClientBoundUpdateUserList struct {
	Users map[string]string `json:"u"`
}
