package server

// User represents a connected player.
type User struct {
	SessionID string
	Username  string
	Token     string
	IGN       string
}

// NewUser creates a user with the default IGN placeholder.
func NewUser(sessionID, username, token string) *User {
	return &User{
		SessionID: sessionID,
		Username:  username,
		Token:     token,
		IGN:       "Unknown",
	}
}
