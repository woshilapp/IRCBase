package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/woshilapp/IRCBaseGo/packets"
)

// Packet represents any encoded message exchanged between the client and server.
type Packet interface{}

// Manager keeps the mapping between packet identifiers and their concrete Go structs.
type Manager struct {
	idToFactory map[int]func() Packet
	typeToID    map[reflect.Type]int
}

// NewManager constructs a Manager with all supported packets registered.
func NewManager() *Manager {
	m := &Manager{
		idToFactory: make(map[int]func() Packet),
		typeToID:    make(map[reflect.Type]int),
	}

	// Keep the identifiers stable so the Go re-write stays protocol compatible
	// with the previous Java implementation.
	m.Register(
		&packets.ClientBoundDisconnect{},
		&packets.ClientBoundConnected{},
		&packets.ClientBoundUpdateUserList{},
		&packets.ClientBoundMessage{},
		&packets.ServerBoundHandshake{},
		&packets.ServerBoundUpdateIGN{},
		&packets.ServerBoundMessage{},
	)

	return m
}

// Register assigns identifiers sequentially for each prototype packet.
func (m *Manager) Register(prototypes ...Packet) {
	nextID := len(m.idToFactory)
	for _, proto := range prototypes {
		typ := reflect.TypeOf(proto)
		if typ.Kind() != reflect.Ptr {
			panic("packet prototypes must be pointers")
		}
		if _, exists := m.typeToID[typ]; exists {
			continue
		}
		id := nextID
		nextID++
		m.idToFactory[id] = func(t reflect.Type) func() Packet {
			return func() Packet {
				return reflect.New(t.Elem()).Interface()
			}
		}(typ)
		m.typeToID[typ] = id
	}
}

// Encode serialises the packet into the on-the-wire representation.
func (m *Manager) Encode(packet Packet) ([]byte, error) {
	if packet == nil {
		return nil, errors.New("cannot encode nil packet")
	}
	typ := reflect.TypeOf(packet)
	id, ok := m.typeToID[typ]
	if !ok {
		return nil, fmt.Errorf("unregistered packet type %s", typ)
	}
	ctx, err := json.Marshal(packet)
	if err != nil {
		return nil, fmt.Errorf("encode packet payload: %w", err)
	}
	envelope := struct {
		ID  int             `json:"id"`
		Ctx json.RawMessage `json:"cxt"`
	}{ID: id, Ctx: ctx}

	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode envelope: %w", err)
	}
	return data, nil
}

// Decode converts raw bytes into a concrete packet instance.
func (m *Manager) Decode(data []byte) (Packet, error) {
	var envelope struct {
		ID  int             `json:"id"`
		Ctx json.RawMessage `json:"cxt"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	factory, ok := m.idToFactory[envelope.ID]
	if !ok {
		return nil, fmt.Errorf("unknown packet id %d", envelope.ID)
	}
	packet := factory()
	if err := json.Unmarshal(envelope.Ctx, packet); err != nil {
		return nil, fmt.Errorf("decode packet payload: %w", err)
	}
	return packet, nil
}
