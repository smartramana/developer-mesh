package webhook_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"testing"
)

func TestWebhook(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping functional tests in short mode - requires services to be running")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook Suite")
}
