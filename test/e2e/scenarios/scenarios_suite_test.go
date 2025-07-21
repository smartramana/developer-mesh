package scenarios

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestScenarios(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Scenarios Suite")
}
