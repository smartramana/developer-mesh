package mcp_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMcp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional tests in short mode - requires services to be running")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Protocol Suite")
}