package rest_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"functional-tests/shared"
)

var (
	config  *shared.ServiceConfig
	restURL string
)

func TestREST(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "REST API Suite")
}

var _ = BeforeSuite(func() {
	config = shared.GetTestConfig()
	restURL = config.RestAPIURL
})
