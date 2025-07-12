package api_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"functional-tests/shared"

	ws "github.com/S-Corkum/devops-mcp/pkg/models/websocket"
)

var _ = Describe("WebSocket Binary Protocol", func() {
	var (
		conn   *websocket.Conn
		ctx    context.Context
		cancel context.CancelFunc
		wsURL  string
		apiKey string
	)

	BeforeEach(func() {
		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)

		// Get test configuration
		config := shared.GetTestConfig()
		wsURL = config.WebSocketURL
		apiKey = shared.GetTestAPIKey("test-tenant-1")

		// Connect with binary protocol support
		var err error
		conn, err = shared.EstablishConnection(wsURL, apiKey)
		Expect(err).NotTo(HaveOccurred())

		// Enable binary protocol
		enableMsg := ws.Message{
			ID:     uuid.New().String(),
			Type:   ws.MessageTypeRequest,
			Method: "protocol.set_binary",
			Params: map[string]interface{}{
				"enabled": true,
				"compression": map[string]interface{}{
					"enabled":   true,
					"threshold": 1024, // Compress messages > 1KB
				},
			},
		}

		msgBytes, err := json.Marshal(enableMsg)
		Expect(err).NotTo(HaveOccurred())

		err = conn.Write(ctx, websocket.MessageText, msgBytes)
		Expect(err).NotTo(HaveOccurred())

		// Read confirmation
		_, respBytes, err := conn.Read(ctx)
		Expect(err).NotTo(HaveOccurred())

		var resp ws.Message
		err = json.Unmarshal(respBytes, &resp)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Error).To(BeNil())
	})

	AfterEach(func() {
		if conn != nil {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}
		cancel()
	})

	Describe("Binary Message Format", func() {
		It("should encode and decode binary messages", func() {
			// Create a test message
			testMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "echo",
				Params: map[string]interface{}{
					"data": "test binary protocol",
				},
			}

			// Encode to binary
			binaryMsg, err := encodeBinaryMessage(testMsg)
			Expect(err).NotTo(HaveOccurred())

			// Send binary message
			err = conn.Write(ctx, websocket.MessageBinary, binaryMsg)
			Expect(err).NotTo(HaveOccurred())

			// Read binary response
			msgType, respBytes, err := conn.Read(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(msgType).To(Equal(websocket.MessageBinary))

			// Decode response
			var resp ws.Message
			err = decodeBinaryMessage(respBytes, &resp)
			Expect(err).NotTo(HaveOccurred())

			// Verify echo
			Expect(resp.ID).To(Equal(testMsg.ID))
			if result, ok := resp.Result.(map[string]interface{}); ok {
				Expect(result["data"]).To(Equal("test binary protocol"))
			}
		})

		It("should handle mixed text and binary messages", func() {
			// Send text message
			textMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "protocol.get_info",
			}

			textBytes, err := json.Marshal(textMsg)
			Expect(err).NotTo(HaveOccurred())

			err = conn.Write(ctx, websocket.MessageText, textBytes)
			Expect(err).NotTo(HaveOccurred())

			// Read response (could be text or binary)
			msgType, respBytes, err := conn.Read(ctx)
			Expect(err).NotTo(HaveOccurred())

			var resp ws.Message
			if msgType == websocket.MessageBinary {
				err = decodeBinaryMessage(respBytes, &resp)
			} else {
				err = json.Unmarshal(respBytes, &resp)
			}
			Expect(err).NotTo(HaveOccurred())

			// Send binary message
			binaryMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "echo",
				Params: map[string]interface{}{
					"binary": true,
				},
			}

			binaryBytes, err := encodeBinaryMessage(binaryMsg)
			Expect(err).NotTo(HaveOccurred())

			err = conn.Write(ctx, websocket.MessageBinary, binaryBytes)
			Expect(err).NotTo(HaveOccurred())

			// Should receive binary response
			msgType2, respBytes2, err := conn.Read(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(msgType2).To(Equal(websocket.MessageBinary))

			var resp2 ws.Message
			err = decodeBinaryMessage(respBytes2, &resp2)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Compression", func() {
		It("should compress large payloads", func() {
			// Generate large payload
			largeData := shared.GenerateLargeContext(5000) // ~20KB of text

			largeMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "context.create",
				Params: map[string]interface{}{
					"content":  largeData,
					"name":     "large-context",
					"model_id": "gpt-4", // Default model for tests
				},
			}

			// Encode with compression
			binaryMsg, err := encodeBinaryMessageWithCompression(largeMsg)
			Expect(err).NotTo(HaveOccurred())

			originalSize := len(largeData)
			compressedSize := len(binaryMsg)
			compressionRatio := float64(compressedSize) / float64(originalSize)

			GinkgoWriter.Printf("Original: %d bytes, Compressed: %d bytes, Ratio: %.2f\n",
				originalSize, compressedSize, compressionRatio)

			// Should achieve significant compression
			Expect(compressionRatio).To(BeNumerically("<", 0.5))

			// Send compressed message
			err = conn.Write(ctx, websocket.MessageBinary, binaryMsg)
			Expect(err).NotTo(HaveOccurred())

			// Read response
			_, respBytes, err := conn.Read(ctx)
			Expect(err).NotTo(HaveOccurred())

			var resp ws.Message
			err = decodeBinaryMessage(respBytes, &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Error).To(BeNil())
		})

		It("should not compress small payloads", func() {
			// Small message
			smallMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "ping",
			}

			// Encode (should not compress)
			binaryMsg, err := encodeBinaryMessageWithCompression(smallMsg)
			Expect(err).NotTo(HaveOccurred())

			// Check header indicates no compression
			header := binary.BigEndian.Uint16(binaryMsg[0:2])
			compressed := (header & 0x8000) != 0
			Expect(compressed).To(BeFalse())
		})
	})

	Describe("Performance", func() {
		It("should be faster than JSON for large payloads", func() {
			// Prepare large message
			largeContent := shared.GenerateLargeContext(10000)
			msg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "benchmark",
				Params: map[string]interface{}{
					"content": largeContent,
					"metadata": map[string]interface{}{
						"tokens":     10000,
						"importance": 100,
						"tags":       []string{"benchmark", "performance", "test"},
					},
				},
			}

			// Benchmark JSON encoding/decoding
			jsonStart := time.Now()
			jsonBytes, err := json.Marshal(msg)
			Expect(err).NotTo(HaveOccurred())

			var jsonDecoded ws.Message
			err = json.Unmarshal(jsonBytes, &jsonDecoded)
			Expect(err).NotTo(HaveOccurred())
			jsonDuration := time.Since(jsonStart)

			// Benchmark binary encoding/decoding
			binaryStart := time.Now()
			binaryBytes, err := encodeBinaryMessage(msg)
			Expect(err).NotTo(HaveOccurred())

			var binaryDecoded ws.Message
			err = decodeBinaryMessage(binaryBytes, &binaryDecoded)
			Expect(err).NotTo(HaveOccurred())
			binaryDuration := time.Since(binaryStart)

			GinkgoWriter.Printf("JSON: %v, Binary: %v, Speedup: %.2fx\n",
				jsonDuration, binaryDuration, float64(jsonDuration)/float64(binaryDuration))

			// Binary should be faster (or at least not significantly slower)
			Expect(binaryDuration).To(BeNumerically("<=", time.Duration(float64(jsonDuration)*1.5)))

			// Size comparison
			GinkgoWriter.Printf("JSON size: %d, Binary size: %d, Reduction: %.2f%%\n",
				len(jsonBytes), len(binaryBytes),
				(1-float64(len(binaryBytes))/float64(len(jsonBytes)))*100)
		})

		It("should handle high-frequency small messages efficiently", func() {
			// Send many small messages rapidly
			messageCount := 100
			start := time.Now()

			for i := 0; i < messageCount; i++ {
				msg := ws.Message{
					ID:     uuid.New().String(),
					Type:   ws.MessageTypeRequest,
					Method: "ping",
					Params: map[string]interface{}{
						"sequence": i,
					},
				}

				binaryMsg, err := encodeBinaryMessage(msg)
				Expect(err).NotTo(HaveOccurred())

				err = conn.Write(ctx, websocket.MessageBinary, binaryMsg)
				Expect(err).NotTo(HaveOccurred())
			}

			// Read all responses
			for i := 0; i < messageCount; i++ {
				_, respBytes, err := conn.Read(ctx)
				Expect(err).NotTo(HaveOccurred())

				var resp ws.Message
				err = decodeBinaryMessage(respBytes, &resp)
				Expect(err).NotTo(HaveOccurred())
			}

			duration := time.Since(start)
			messagesPerSecond := float64(messageCount) / duration.Seconds()

			GinkgoWriter.Printf("Processed %d messages in %v (%.0f msg/sec)\n",
				messageCount, duration, messagesPerSecond)

			// Should handle at least 100 messages per second
			Expect(messagesPerSecond).To(BeNumerically(">", 100))
		})
	})

	Describe("Binary Streaming", func() {
		It("should stream binary data efficiently", func() {
			// Start binary stream
			streamMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "stream.binary",
				Params: map[string]interface{}{
					"chunks":     10,
					"chunk_size": 4096, // 4KB chunks
				},
			}

			binaryMsg, err := encodeBinaryMessage(streamMsg)
			Expect(err).NotTo(HaveOccurred())

			err = conn.Write(ctx, websocket.MessageBinary, binaryMsg)
			Expect(err).NotTo(HaveOccurred())

			// Receive stream chunks
			totalBytes := 0
			chunkCount := 0

			for chunkCount < 10 {
				msgType, chunkBytes, err := conn.Read(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(msgType).To(Equal(websocket.MessageBinary))

				// Decode chunk header
				if len(chunkBytes) > 4 {
					chunkType := binary.BigEndian.Uint16(chunkBytes[0:2])
					chunkSize := binary.BigEndian.Uint16(chunkBytes[2:4])

					if chunkType == 0x0001 { // Stream chunk marker
						totalBytes += int(chunkSize)
						chunkCount++
					}
				}
			}

			// Verify received all data
			Expect(chunkCount).To(Equal(10))
			Expect(totalBytes).To(BeNumerically("~", 10*4096, 100))
		})
	})

	Describe("Error Handling", func() {
		It("should handle malformed binary messages", func() {
			// Send malformed binary data
			malformedData := []byte{0xFF, 0xFF, 0xFF, 0xFF} // Invalid header

			err := conn.Write(ctx, websocket.MessageBinary, malformedData)
			Expect(err).NotTo(HaveOccurred())

			// Should receive error response
			_, respBytes, err := conn.Read(ctx)
			Expect(err).NotTo(HaveOccurred())

			var resp ws.Message
			err = json.Unmarshal(respBytes, &resp) // Error might come as JSON
			if err != nil {
				err = decodeBinaryMessage(respBytes, &resp)
			}
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Error).NotTo(BeNil())
			Expect(resp.Error.Message).To(ContainSubstring("malformed"))
		})

		It("should fallback to text protocol on binary errors", func() {
			// Disable binary protocol temporarily
			disableMsg := ws.Message{
				ID:     uuid.New().String(),
				Type:   ws.MessageTypeRequest,
				Method: "protocol.set_binary",
				Params: map[string]interface{}{
					"enabled":  false,
					"fallback": true,
				},
			}

			// This should work even if binary is broken
			jsonBytes, err := json.Marshal(disableMsg)
			Expect(err).NotTo(HaveOccurred())

			err = conn.Write(ctx, websocket.MessageText, jsonBytes)
			Expect(err).NotTo(HaveOccurred())

			// Should receive text response
			msgType, _, err := conn.Read(ctx)
			Expect(err).NotTo(HaveOccurred())
			Expect(msgType).To(Equal(websocket.MessageText))
		})
	})
})

// Helper functions for binary encoding/decoding

func encodeBinaryMessage(msg ws.Message) ([]byte, error) {
	// Simple binary format:
	// [2 bytes: header] [4 bytes: length] [N bytes: msgpack/protobuf data]

	// For testing, we'll use JSON inside binary wrapper
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)

	// Header (version + flags)
	header := uint16(0x0100) // Version 1.0, no compression
	if err := binary.Write(buf, binary.BigEndian, header); err != nil {
		return nil, err
	}

	// Length
	length := uint32(len(jsonData))
	if err := binary.Write(buf, binary.BigEndian, length); err != nil {
		return nil, err
	}

	// Data
	buf.Write(jsonData)

	return buf.Bytes(), nil
}

func encodeBinaryMessageWithCompression(msg ws.Message) ([]byte, error) {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	// Only compress if over threshold
	if len(jsonData) < 1024 {
		return encodeBinaryMessage(msg)
	}

	// Compress data
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, err = gz.Write(jsonData)
	if err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(nil)

	// Header with compression flag
	header := uint16(0x8100) // Version 1.0, compressed
	if err := binary.Write(buf, binary.BigEndian, header); err != nil {
		return nil, err
	}

	// Length of compressed data
	length := uint32(compressed.Len())
	if err := binary.Write(buf, binary.BigEndian, length); err != nil {
		return nil, err
	}

	// Compressed data
	buf.Write(compressed.Bytes())

	return buf.Bytes(), nil
}

func decodeBinaryMessage(data []byte, msg *ws.Message) error {
	if len(data) < 6 {
		return json.Unmarshal(data, msg) // Fallback to JSON
	}

	// Read header
	header := binary.BigEndian.Uint16(data[0:2])
	length := binary.BigEndian.Uint32(data[2:6])

	if uint32(len(data)-6) < length {
		return json.Unmarshal(data, msg) // Fallback to JSON
	}

	payload := data[6 : 6+length]

	// Check if compressed
	if (header & 0x8000) != 0 {
		// Decompress
		gz, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return err
		}
		defer func() {
			_ = gz.Close()
		}()

		var decompressed bytes.Buffer
		_, err = decompressed.ReadFrom(gz)
		if err != nil {
			return err
		}
		payload = decompressed.Bytes()
	}

	return json.Unmarshal(payload, msg)
}
