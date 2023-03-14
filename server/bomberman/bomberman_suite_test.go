package bomberman_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBomberman(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bomberman Suite")
}
