package process_tracker_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestProcess_tracker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Process Tracker Suite")
}
