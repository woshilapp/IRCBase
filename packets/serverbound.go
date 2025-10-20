package packets

// Server bound packets sent from the client to the server.

// ServerBoundHandshake starts the authentication process for a user.
type ServerBoundHandshake struct {
	Username string `json:"u"`
	Token    string `json:"t"`
}

// ServerBoundUpdateIGN informs the server of the client's in game name.
type ServerBoundUpdateIGN struct {
	Name string `json:"n"`
}

// ServerBoundMessage carries a chat message from the client.
type ServerBoundMessage struct {
	Message string `json:"m"`
}
