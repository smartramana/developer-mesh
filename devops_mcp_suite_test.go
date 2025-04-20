package devops_mcp_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestDevopsMcp(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DevopsMcp Suite")
}
