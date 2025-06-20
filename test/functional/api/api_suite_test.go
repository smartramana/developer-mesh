package api_test

import (
	"testing"

	"github.com/joho/godotenv"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestApi(t *testing.T) {
	// Load .env file
	_ = godotenv.Load("../.env")

	if testing.Short() {
		t.Skip("Skipping functional tests in short mode - requires services to be running")
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Api Suite")
}
