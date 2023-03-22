package timebomb_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestTimeBomb(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TimeBomb Suite")
}
