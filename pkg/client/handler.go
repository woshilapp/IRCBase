package client

// Handler reacts to client transport events.
type Handler interface {
	OnMessage(sender, message string)
	OnDisconnected(reason string)
	OnConnected()
	InGameUsername() string
}
