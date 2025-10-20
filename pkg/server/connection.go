package server

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"sync"

	"github.com/cubk/ircbase/pkg/packets"
	"github.com/cubk/ircbase/pkg/protocol"
)

var errConnectionClosed = errors.New("connection closed")

// connection wraps a TCP connection and handles framing + codec interactions.
type connection struct {
	id     string
	conn   net.Conn
	server *Server

	send      chan []byte
	closeOnce sync.Once
	done      chan struct{}
}

func newConnection(id string, conn net.Conn, server *Server) *connection {
	c := &connection{
		id:     id,
		conn:   conn,
		server: server,
		send:   make(chan []byte, 16),
		done:   make(chan struct{}),
	}
	go c.writeLoop()
	return c
}

func (c *connection) readLoop() {
	reader := bufio.NewReader(c.conn)
	for {
		var lengthBytes [4]byte
		if _, err := io.ReadFull(reader, lengthBytes[:]); err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("connection %s read error: %v", c.id, err)
			}
			c.close()
			return
		}
		length := binary.BigEndian.Uint32(lengthBytes[:])
		if length == 0 {
			continue
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(reader, payload); err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("connection %s payload error: %v", c.id, err)
			}
			c.close()
			return
		}
		packet, err := c.server.codec.Decode(payload)
		if err != nil {
			log.Printf("connection %s decode error: %v", c.id, err)
			_ = c.SendPacket(&packets.ClientBoundDisconnect{Reason: "协议错误"})
			c.close()
			return
		}
		c.server.handlePacket(c, packet)
	}
}

func (c *connection) writeLoop() {
	for {
		select {
		case <-c.done:
			return
		case data := <-c.send:
			if data == nil {
				continue
			}
			if _, err := c.conn.Write(data); err != nil {
				log.Printf("connection %s write error: %v", c.id, err)
				c.close()
				return
			}
		}
	}
}

// SendPacket queues the packet to be written to the TCP connection.
func (c *connection) SendPacket(packet protocol.Packet) error {
	data, err := c.server.codec.Encode(packet)
	if err != nil {
		return err
	}
	framed := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(framed[:4], uint32(len(data)))
	copy(framed[4:], data)

	select {
	case <-c.done:
		return errConnectionClosed
	default:
	}

	select {
	case c.send <- framed:
		return nil
	case <-c.done:
		return errConnectionClosed
	}
}

func (c *connection) Disconnect(reason string) {
	if reason != "" {
		_ = c.SendPacket(&packets.ClientBoundDisconnect{Reason: reason})
	}
	c.close()
}

func (c *connection) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
		c.server.onConnectionClosed(c)
	})
}
