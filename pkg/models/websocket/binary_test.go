package websocket

import (
    "bytes"
    "encoding/json"
    "testing"
    
    "github.com/stretchr/testify/assert"
)

func TestBinaryProtocol(t *testing.T) {
    // Test header parsing
    header := &BinaryHeader{
        Magic:      MagicNumber,
        Version:    1,
        Type:       MessageTypeRequest.ToByte(),
        SequenceID: 12345,
        Method:     MethodToolList,
        DataSize:   100,
    }
    
    buf := &bytes.Buffer{}
    err := WriteBinaryHeader(buf, header)
    assert.NoError(t, err)
    
    parsed, err := ParseBinaryHeader(buf)
    assert.NoError(t, err)
    
    assert.Equal(t, header.Magic, parsed.Magic)
    assert.Equal(t, header.Version, parsed.Version)
    assert.Equal(t, header.Type, parsed.Type)
    assert.Equal(t, header.SequenceID, parsed.SequenceID)
    assert.Equal(t, header.Method, parsed.Method)
    assert.Equal(t, header.DataSize, parsed.DataSize)
}

func TestBinaryHeaderValidation(t *testing.T) {
    tests := []struct {
        name    string
        header  BinaryHeader
        wantErr error
    }{
        {
            name: "Invalid magic",
            header: BinaryHeader{
                Magic:   0x12345678,
                Version: 1,
            },
            wantErr: ErrInvalidMagic,
        },
        {
            name: "Unsupported version",
            header: BinaryHeader{
                Magic:   MagicNumber,
                Version: 99,
            },
            wantErr: ErrUnsupportedVersion,
        },
        {
            name: "Payload too large",
            header: BinaryHeader{
                Magic:    MagicNumber,
                Version:  1,
                DataSize: 2 * 1024 * 1024, // 2MB
            },
            wantErr: ErrPayloadTooLarge,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            buf := &bytes.Buffer{}
            err := WriteBinaryHeader(buf, &tt.header)
            assert.NoError(t, err)
            
            _, err = ParseBinaryHeader(buf)
            assert.Equal(t, tt.wantErr, err)
        })
    }
}

func TestBinaryFlags(t *testing.T) {
    header := &BinaryHeader{}
    
    // Test setting flags
    header.SetFlag(FlagCompressed)
    assert.True(t, header.IsCompressed())
    assert.False(t, header.IsEncrypted())
    
    header.SetFlag(FlagEncrypted)
    assert.True(t, header.IsCompressed())
    assert.True(t, header.IsEncrypted())
    
    // Test clearing flags
    header.ClearFlag(FlagCompressed)
    assert.False(t, header.IsCompressed())
    assert.True(t, header.IsEncrypted())
    
    // Test batch flag
    header.SetFlag(FlagBatch)
    assert.True(t, header.IsBatch())
}

func TestMethodConversion(t *testing.T) {
    tests := []struct {
        method uint16
        str    string
    }{
        {MethodInitialize, "initialize"},
        {MethodToolList, "tool.list"},
        {MethodToolExecute, "tool.execute"},
        {MethodContextGet, "context.get"},
        {MethodContextUpdate, "context.update"},
        {MethodEventSubscribe, "event.subscribe"},
        {MethodEventUnsubscribe, "event.unsubscribe"},
        {MethodPing, "ping"},
        {MethodPong, "pong"},
        {999, "unknown"},
    }
    
    for _, tt := range tests {
        t.Run(tt.str, func(t *testing.T) {
            assert.Equal(t, tt.str, MethodToString(tt.method))
            if tt.method != 999 {
                assert.Equal(t, tt.method, StringToMethod(tt.str))
            }
        })
    }
}

func TestMessageTypeConversion(t *testing.T) {
    types := []MessageType{
        MessageTypeRequest,
        MessageTypeResponse,
        MessageTypeNotification,
        MessageTypeError,
        MessageTypePing,
        MessageTypePong,
    }
    
    for _, typ := range types {
        b := typ.ToByte()
        converted := MessageTypeFromByte(b)
        assert.Equal(t, typ, converted)
    }
}

func BenchmarkBinaryParsing(b *testing.B) {
    // Create test message
    header := &BinaryHeader{
        Magic:      MagicNumber,
        Version:    1,
        Type:       MessageTypeRequest.ToByte(),
        Method:     MethodToolExecute,
        SequenceID: 12345,
        DataSize:   100,
    }
    
    buf := &bytes.Buffer{}
    if err := WriteBinaryHeader(buf, header); err != nil {
        b.Fatalf("Failed to write header: %v", err)
    }
    data := buf.Bytes()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        r := bytes.NewReader(data)
        _, _ = ParseBinaryHeader(r)
    }
}

func BenchmarkBinaryWriting(b *testing.B) {
    header := &BinaryHeader{
        Magic:      MagicNumber,
        Version:    1,
        Type:       MessageTypeRequest.ToByte(),
        Method:     MethodToolExecute,
        SequenceID: 12345,
        DataSize:   100,
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        buf := &bytes.Buffer{}
        _ = WriteBinaryHeader(buf, header)
    }
}

// BenchmarkComparison compares binary vs JSON performance
func BenchmarkComparison(b *testing.B) {
    // Binary message
    binaryHeader := &BinaryHeader{
        Magic:      MagicNumber,
        Version:    1,
        Type:       MessageTypeRequest.ToByte(),
        Method:     MethodToolExecute,
        SequenceID: 12345,
        DataSize:   0,
    }
    
    // JSON message
    jsonMsg := &Message{
        ID:     "12345",
        Type:   MessageTypeRequest,
        Method: "tool.execute",
        Params: map[string]string{"tool": "test"},
    }
    
    b.Run("Binary", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            buf := &bytes.Buffer{}
            _ = WriteBinaryHeader(buf, binaryHeader)
            _, _ = ParseBinaryHeader(buf)
        }
    })
    
    b.Run("JSON", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            data, _ := json.Marshal(jsonMsg)
            var msg Message
            _ = json.Unmarshal(data, &msg)
        }
    })
}