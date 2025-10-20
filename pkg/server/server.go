package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cubk/ircbase/pkg/packets"
	"github.com/cubk/ircbase/pkg/protocol"
)

// Server hosts the chat service.
type Server struct {
	addr   string
	codec  *protocol.Manager
	users  *UserManager
	mu     sync.RWMutex
	conns  map[string]*connection
	nextID atomic.Uint64
}

// New creates a Server bound to the provided address.
func New(addr string) *Server {
	return &Server{
		addr:  addr,
		codec: protocol.NewManager(),
		users: NewUserManager(),
		conns: make(map[string]*connection),
	}
}

// Run starts accepting incoming TCP connections until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	log.Printf("IRC server listening on %s", s.addr)

	go s.periodicUserSync(ctx)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			log.Printf("accept error: %v", err)
			continue
		}
		sessionID := fmt.Sprintf("%x", s.nextID.Add(1))
		c := newConnection(sessionID, conn, s)
		s.mu.Lock()
		s.conns[sessionID] = c
		s.mu.Unlock()
		log.Printf("client %s connected from %s", sessionID, conn.RemoteAddr())
		go c.readLoop()
	}
}

func (s *Server) onConnectionClosed(c *connection) {
	s.mu.Lock()
	delete(s.conns, c.id)
	s.mu.Unlock()
	if user, ok := s.users.Get(c.id); ok {
		log.Printf("user %s disconnected", user.Username)
		s.users.Remove(c.id)
		s.broadcastUserList()
	}
}

func (s *Server) handlePacket(c *connection, pkt protocol.Packet) {
	switch p := pkt.(type) {
	case *packets.ServerBoundHandshake:
		s.handleHandshake(c, p)
	case *packets.ServerBoundUpdateIGN:
		s.handleUpdateIGN(c, p)
	case *packets.ServerBoundMessage:
		s.handleMessage(c, p)
	default:
		log.Printf("no handler for packet %T", pkt)
	}
}

func (s *Server) handleHandshake(c *connection, pkt *packets.ServerBoundHandshake) {
	if _, exists := s.users.Get(c.id); exists {
		_ = c.SendPacket(&packets.ClientBoundDisconnect{Reason: "你已经连接到了这个服务器！"})
		return
	}
	user := NewUser(c.id, pkt.Username, pkt.Token)
	s.users.Put(c.id, user)
	if err := c.SendPacket(&packets.ClientBoundConnected{}); err != nil {
		log.Printf("failed to acknowledge connection for %s: %v", pkt.Username, err)
		return
	}
	log.Printf("accepted user %s (%s)", pkt.Username, c.conn.RemoteAddr())
}

func (s *Server) handleUpdateIGN(c *connection, pkt *packets.ServerBoundUpdateIGN) {
	user, ok := s.users.Get(c.id)
	if !ok {
		log.Printf("update ign for unknown session %s", c.id)
		return
	}
	prev := user.IGN
	if prev == pkt.Name {
		return
	}
	user.IGN = pkt.Name
	log.Printf("user %s updated IGN %s -> %s", user.Username, prev, user.IGN)
	s.broadcastUserList()
}

func (s *Server) handleMessage(c *connection, pkt *packets.ServerBoundMessage) {
	user, ok := s.users.Get(c.id)
	if !ok {
		log.Printf("message from unknown session %s", c.id)
		return
	}
	s.Broadcast(&packets.ClientBoundMessage{Sender: user.Username, Message: pkt.Message})
	log.Printf("chat %s >> %s", user.Username, pkt.Message)
}

// Broadcast sends the packet to every connected client.
func (s *Server) Broadcast(packet protocol.Packet) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, conn := range s.conns {
		if err := conn.SendPacket(packet); err != nil {
			log.Printf("broadcast to %s failed: %v", conn.id, err)
		}
	}
}

func (s *Server) broadcastUserList() {
	snapshot := s.users.Snapshot()
	s.Broadcast(&packets.ClientBoundUpdateUserList{Users: snapshot})
}

func (s *Server) periodicUserSync(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.broadcastUserList()
		case <-ctx.Done():
			return
		}
	}
}
