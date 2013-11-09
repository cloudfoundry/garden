package linux_container_pool_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestContainer_pool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Container_pool Suite")
}
