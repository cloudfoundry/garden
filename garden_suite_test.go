package garden_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGarden(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Garden Suite")
}
