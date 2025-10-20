package client

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/cubk/ircbase/pkg/packets"
	"github.com/cubk/ircbase/pkg/protocol"
)

var errConnectionClosed = errors.New("connection closed")

// Transport manages the connection to the IRC server.
type Transport struct {
	conn    net.Conn
	codec   *protocol.Manager
	handler Handler

	send chan []byte
	done chan struct{}

	mu         sync.RWMutex
	userToIGN  map[string]string
	ignToUser  map[string]string
	tickerOnce sync.Once
}

// NewTransport connects to the server at the provided address.
func NewTransport(addr string, handler Handler) (*Transport, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	t := &Transport{
		conn:      conn,
		codec:     protocol.NewManager(),
		handler:   handler,
		send:      make(chan []byte, 16),
		done:      make(chan struct{}),
		userToIGN: make(map[string]string),
		ignToUser: make(map[string]string),
	}
	go t.writeLoop()
	go t.readLoop()
	return t, nil
}

func (t *Transport) readLoop() {
	reader := bufio.NewReader(t.conn)
	for {
		var lengthBytes [4]byte
		if _, err := io.ReadFull(reader, lengthBytes[:]); err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("client read error: %v", err)
			}
			t.Close()
			t.notifyDisconnected("连接已关闭")
			return
		}
		length := binary.BigEndian.Uint32(lengthBytes[:])
		if length == 0 {
			continue
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(reader, payload); err != nil {
			log.Printf("client payload error: %v", err)
			t.Close()
			t.notifyDisconnected("连接已关闭")
			return
		}
		packet, err := t.codec.Decode(payload)
		if err != nil {
			log.Printf("client decode error: %v", err)
			t.Close()
			t.notifyDisconnected("协议错误")
			return
		}
		t.handlePacket(packet)
	}
}

func (t *Transport) writeLoop() {
	for {
		select {
		case data, ok := <-t.send:
			if !ok {
				return
			}
			if _, err := t.conn.Write(data); err != nil {
				log.Printf("client write error: %v", err)
				t.Close()
				return
			}
		case <-t.done:
			return
		}
	}
}

func (t *Transport) handlePacket(pkt protocol.Packet) {
	switch p := pkt.(type) {
	case *packets.ClientBoundDisconnect:
		t.notifyDisconnected(p.Reason)
		t.Close()
	case *packets.ClientBoundConnected:
		t.notifyConnected()
		t.tickerOnce.Do(func() {
			go t.periodicIGNUpdates()
		})
	case *packets.ClientBoundUpdateUserList:
		t.mu.Lock()
		t.userToIGN = make(map[string]string, len(p.Users))
		t.ignToUser = make(map[string]string, len(p.Users))
		for user, ign := range p.Users {
			t.userToIGN[user] = ign
			t.ignToUser[ign] = user
		}
		t.mu.Unlock()
	case *packets.ClientBoundMessage:
		t.notifyMessage(p.Sender, p.Message)
	default:
		log.Printf("client received unknown packet %T", pkt)
	}
}

func (t *Transport) handlerSnapshot() Handler {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.handler
}

func (t *Transport) notifyDisconnected(reason string) {
	if handler := t.handlerSnapshot(); handler != nil {
		handler.OnDisconnected(reason)
	}
}

func (t *Transport) notifyConnected() {
	if handler := t.handlerSnapshot(); handler != nil {
		handler.OnConnected()
	}
}

func (t *Transport) notifyMessage(sender, message string) {
	if handler := t.handlerSnapshot(); handler != nil {
		handler.OnMessage(sender, message)
	}
}

func (t *Transport) periodicIGNUpdates() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			handler := t.handlerSnapshot()
			if handler == nil {
				continue
			}
			ign := handler.InGameUsername()
			if ign != "" {
				t.SendInGameUsername(ign)
			}
		case <-t.done:
			return
		}
	}
}

// Connect sends the handshake packet with username and token.
func (t *Transport) Connect(username, token string) {
	_ = t.SendPacket(&packets.ServerBoundHandshake{Username: username, Token: token})
}

// SendChat submits a chat message to the server.
func (t *Transport) SendChat(message string) {
	_ = t.SendPacket(&packets.ServerBoundMessage{Message: message})
}

// SendInGameUsername updates the user's in-game-name.
func (t *Transport) SendInGameUsername(name string) {
	_ = t.SendPacket(&packets.ServerBoundUpdateIGN{Name: name})
}

// SendPacket encodes the packet and queues it for sending.
func (t *Transport) SendPacket(packet protocol.Packet) error {
	data, err := t.codec.Encode(packet)
	if err != nil {
		return err
	}
	framed := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(framed[:4], uint32(len(data)))
	copy(framed[4:], data)

	select {
	case t.send <- framed:
		return nil
	case <-t.done:
		return errConnectionClosed
	}
}

// IsUser returns true if the username is known.
func (t *Transport) IsUser(name string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.userToIGN[name]
	return ok
}

// GetName looks up the username for a known IGN.
func (t *Transport) GetName(ign string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	name, ok := t.ignToUser[ign]
	return name, ok
}

// GetIGN retrieves the IGN for a username.
func (t *Transport) GetIGN(name string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ign, ok := t.userToIGN[name]
	return ign, ok
}

// SetHandler swaps the handler at runtime.
func (t *Transport) SetHandler(handler Handler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handler = handler
}

// Close terminates the connection.
func (t *Transport) Close() {
	select {
	case <-t.done:
		return
	default:
	}
	close(t.done)
	close(t.send)
	_ = t.conn.Close()
}
