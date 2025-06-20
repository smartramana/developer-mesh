package websocket

import (
	"encoding/binary"
	"errors"
	"io"
)

// BinaryHeader represents the binary protocol header (24 bytes)
type BinaryHeader struct {
	Magic      uint32 // 0x4D435057 "MCPW"
	Version    uint8  // Protocol version (1)
	Type       uint8  // Message type
	Flags      uint16 // Compression, encryption flags
	SequenceID uint64 // Message sequence ID
	Method     uint16 // Method enum (not string)
	Reserved   uint16 // Padding for alignment
	DataSize   uint32 // Payload size
}

// Method enums for binary protocol
const (
	MethodInitialize       uint16 = 1
	MethodToolList         uint16 = 2
	MethodToolExecute      uint16 = 3
	MethodContextGet       uint16 = 4
	MethodContextUpdate    uint16 = 5
	MethodEventSubscribe   uint16 = 6
	MethodEventUnsubscribe uint16 = 7
	MethodPing             uint16 = 8
	MethodPong             uint16 = 9
)

// Flag bits
const (
	FlagCompressed uint16 = 1 << 0
	FlagEncrypted  uint16 = 1 << 1
	FlagBatch      uint16 = 1 << 2
)

// Magic number for protocol identification
const MagicNumber uint32 = 0x4D435057 // "MCPW" in hex

// Errors
var (
	ErrInvalidMagic       = errors.New("invalid magic number")
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
	ErrPayloadTooLarge    = errors.New("payload too large")
	ErrInvalidHeader      = errors.New("invalid header")
)

// ParseBinaryHeader reads and validates a binary header
func ParseBinaryHeader(r io.Reader) (*BinaryHeader, error) {
	header := &BinaryHeader{}

	// Read 24 bytes
	buf := make([]byte, 24)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}

	// Parse fields
	header.Magic = binary.BigEndian.Uint32(buf[0:4])
	header.Version = buf[4]
	header.Type = buf[5]
	header.Flags = binary.BigEndian.Uint16(buf[6:8])
	header.SequenceID = binary.BigEndian.Uint64(buf[8:16])
	header.Method = binary.BigEndian.Uint16(buf[16:18])
	header.Reserved = binary.BigEndian.Uint16(buf[18:20])
	header.DataSize = binary.BigEndian.Uint32(buf[20:24])

	// Validate
	if header.Magic != MagicNumber {
		return nil, ErrInvalidMagic
	}

	if header.Version != 1 {
		return nil, ErrUnsupportedVersion
	}

	if header.DataSize > 1024*1024 { // 1MB max
		return nil, ErrPayloadTooLarge
	}

	return header, nil
}

// WriteBinaryHeader writes a binary header
func WriteBinaryHeader(w io.Writer, header *BinaryHeader) error {
	buf := make([]byte, 24)

	binary.BigEndian.PutUint32(buf[0:4], header.Magic)
	buf[4] = header.Version
	buf[5] = header.Type
	binary.BigEndian.PutUint16(buf[6:8], header.Flags)
	binary.BigEndian.PutUint64(buf[8:16], header.SequenceID)
	binary.BigEndian.PutUint16(buf[16:18], header.Method)
	binary.BigEndian.PutUint16(buf[18:20], header.Reserved)
	binary.BigEndian.PutUint32(buf[20:24], header.DataSize)

	_, err := w.Write(buf)
	return err
}

// IsCompressed checks if the compressed flag is set
func (h *BinaryHeader) IsCompressed() bool {
	return h.Flags&FlagCompressed != 0
}

// IsEncrypted checks if the encrypted flag is set
func (h *BinaryHeader) IsEncrypted() bool {
	return h.Flags&FlagEncrypted != 0
}

// IsBatch checks if the batch flag is set
func (h *BinaryHeader) IsBatch() bool {
	return h.Flags&FlagBatch != 0
}

// SetFlag sets a flag bit
func (h *BinaryHeader) SetFlag(flag uint16) {
	h.Flags |= flag
}

// ClearFlag clears a flag bit
func (h *BinaryHeader) ClearFlag(flag uint16) {
	h.Flags &^= flag
}

// MethodToString converts a method enum to string
func MethodToString(method uint16) string {
	switch method {
	case MethodInitialize:
		return "initialize"
	case MethodToolList:
		return "tool.list"
	case MethodToolExecute:
		return "tool.execute"
	case MethodContextGet:
		return "context.get"
	case MethodContextUpdate:
		return "context.update"
	case MethodEventSubscribe:
		return "event.subscribe"
	case MethodEventUnsubscribe:
		return "event.unsubscribe"
	case MethodPing:
		return "ping"
	case MethodPong:
		return "pong"
	default:
		return "unknown"
	}
}

// StringToMethod converts a string method to enum
func StringToMethod(method string) uint16 {
	switch method {
	case "initialize":
		return MethodInitialize
	case "tool.list":
		return MethodToolList
	case "tool.execute":
		return MethodToolExecute
	case "context.get":
		return MethodContextGet
	case "context.update":
		return MethodContextUpdate
	case "event.subscribe":
		return MethodEventSubscribe
	case "event.unsubscribe":
		return MethodEventUnsubscribe
	case "ping":
		return MethodPing
	case "pong":
		return MethodPong
	default:
		return 0
	}
}

// MessageTypeFromByte converts a byte to MessageType
func MessageTypeFromByte(b uint8) MessageType {
	return MessageType(b)
}

// ToByte converts MessageType to byte
func (t MessageType) ToByte() uint8 {
	return uint8(t)
}
