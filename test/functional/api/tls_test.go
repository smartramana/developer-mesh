package api_test

import (
	"crypto/tls"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TLS Configuration Tests", func() {
	Context("when TLS is enabled", func() {
		var (
			tlsEnabled bool
			tlsAPIURL  string
			tlsWSURL   string
		)

		BeforeEach(func() {
			// Check if TLS testing is enabled via environment variable
			tlsEnabled = os.Getenv("TEST_TLS_ENABLED") == "true"
			if !tlsEnabled {
				Skip("TLS testing is disabled. Set TEST_TLS_ENABLED=true to run TLS tests")
			}

			// Get TLS endpoints from environment or use defaults
			tlsAPIURL = os.Getenv("TEST_TLS_API_URL")
			if tlsAPIURL == "" {
				tlsAPIURL = "https://localhost:8443"
			}

			tlsWSURL = os.Getenv("TEST_TLS_WS_URL")
			if tlsWSURL == "" {
				tlsWSURL = "wss://localhost:8443/ws"
			}
		})

		It("should enforce minimum TLS version 1.2", func() {
			// Create a client that only supports TLS 1.1
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						MaxVersion: tls.VersionTLS11,
						InsecureSkipVerify: true, // For self-signed certs in testing
					},
				},
				Timeout: 5 * time.Second,
			}

			// Attempt to connect with TLS 1.1
			_, err := client.Get(tlsAPIURL + "/health")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("protocol version"))
		})

		It("should accept TLS 1.2 connections", func() {
			// Create a client that uses TLS 1.2
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						MinVersion: tls.VersionTLS12,
						MaxVersion: tls.VersionTLS12,
						InsecureSkipVerify: true, // For self-signed certs in testing
					},
				},
				Timeout: 5 * time.Second,
			}

			// Connect with TLS 1.2
			resp, err := client.Get(tlsAPIURL + "/health")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			resp.Body.Close()
		})

		It("should prefer TLS 1.3 connections", func() {
			// Create a client that supports TLS 1.3
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						MinVersion: tls.VersionTLS13,
						InsecureSkipVerify: true, // For self-signed certs in testing
					},
				},
				Timeout: 5 * time.Second,
			}

			// Connect with TLS 1.3
			resp, err := client.Get(tlsAPIURL + "/health")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			
			// Verify we're using TLS 1.3
			if resp.TLS != nil {
				Expect(resp.TLS.Version).To(Equal(uint16(tls.VersionTLS13)))
			}
			resp.Body.Close()
		})

		It("should use secure cipher suites only", func() {
			// Create a client with weak cipher suites
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						MinVersion: tls.VersionTLS12,
						CipherSuites: []uint16{
							tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA, // Weak cipher
							tls.TLS_RSA_WITH_RC4_128_SHA,      // Weak cipher
						},
						InsecureSkipVerify: true,
					},
				},
				Timeout: 5 * time.Second,
			}

			// Attempt to connect with weak ciphers
			_, err := client.Get(tlsAPIURL + "/health")
			Expect(err).To(HaveOccurred())
			// The error should indicate no cipher suite match
		})

		It("should support secure cipher suites", func() {
			// Create a client with secure cipher suites
			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						MinVersion: tls.VersionTLS12,
						CipherSuites: []uint16{
							tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
							tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
							tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
							tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
						},
						InsecureSkipVerify: true,
					},
				},
				Timeout: 5 * time.Second,
			}

			// Connect with secure ciphers
			resp, err := client.Get(tlsAPIURL + "/health")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			resp.Body.Close()
		})
	})

	Context("TLS configuration documentation", func() {
		It("should provide clear instructions for enabling TLS testing", func() {
			if os.Getenv("TEST_TLS_ENABLED") != "true" {
				GinkgoWriter.Printf(`
TLS Testing Instructions:
========================

To run TLS tests, you need to:

1. Generate test certificates:
   ./scripts/certs/generate-dev-certs.sh

2. Set environment variables:
   export TEST_TLS_ENABLED=true
   export TLS_CERT_FILE=./certs/server.crt
   export TLS_KEY_FILE=./certs/server.key
   export TLS_CA_FILE=./certs/ca.crt

3. Start services with TLS enabled:
   # Update config files to enable TLS
   # Or use environment variables to override

4. Run TLS tests:
   TEST_TLS_ENABLED=true ginkgo ./test/functional/api -- --ginkgo.focus="TLS"

For production testing:
- Use certificates from cert-manager.io
- Ensure proper certificate validation
- Test with real domain names
`)
			}
		})
	})
})