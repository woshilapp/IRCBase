package server

import (
	"sort"
	"sync"
)

// UserManager keeps track of active users.
type UserManager struct {
	mu    sync.RWMutex
	users map[string]*User
}

// NewUserManager creates an empty user manager.
func NewUserManager() *UserManager {
	return &UserManager{users: make(map[string]*User)}
}

// Put stores or replaces the user for the given session.
func (m *UserManager) Put(session string, user *User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[session] = user
}

// Remove forgets a user by session id.
func (m *UserManager) Remove(session string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.users, session)
}

// Get fetches the user for a session.
func (m *UserManager) Get(session string) (*User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.users[session]
	return user, ok
}

// Snapshot returns an ordered copy of username -> IGN data.
func (m *UserManager) Snapshot() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	type pair struct {
		username string
		ign      string
	}
	pairs := make([]pair, 0, len(m.users))
	for _, user := range m.users {
		pairs = append(pairs, pair{username: user.Username, ign: user.IGN})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].username < pairs[j].username
	})
	snapshot := make(map[string]string, len(pairs))
	for _, p := range pairs {
		snapshot[p.username] = p.ign
	}
	return snapshot
}

// Users returns every stored user.
func (m *UserManager) Users() []*User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*User, 0, len(m.users))
	for _, user := range m.users {
		out = append(out, user)
	}
	return out
}
