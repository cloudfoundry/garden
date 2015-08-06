package streamer_test

import (
	"bytes"
	"errors"
	"sync"
	"time"

	"runtime"

	"github.com/cloudfoundry-incubator/garden/server/streamer"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Streamer", func() {

	const testString = "x"

	var (
		graceTime time.Duration
		str       *streamer.Streamer

		stdoutChan        chan []byte
		stderrChan        chan []byte
		testByteSlice     []byte
		channelBufferSize int
	)

	JustBeforeEach(func() {
		str = streamer.New(graceTime)
		stdoutChan = make(chan []byte, channelBufferSize)
		stderrChan = make(chan []byte, channelBufferSize)
	})

	BeforeEach(func() {
		graceTime = 10 * time.Second
		channelBufferSize = 1

		testByteSlice = []byte(testString)
	})

	It("should stream standard output until it is stopped", func() {
		sid := str.Stream(stdoutChan, stderrChan)
		w := &syncBuffer{
			Buffer: new(bytes.Buffer),
		}
		go str.ServeStdout(sid, w)
		stdoutChan <- testByteSlice
		stdoutChan <- testByteSlice
		Eventually(w.String).Should(Equal("xx"))
		str.Stop(sid)
	})

	// The following test will not reliably fail if the implementation fails to drain messages.
	It("should stream the remaining standard output messages after being stopped", func() {
		sid := str.Stream(stdoutChan, stderrChan)
		str.Stop(sid)
		w := new(bytes.Buffer)
		stdoutChan <- testByteSlice
		str.ServeStdout(sid, w)
		Expect(w.String()).To(Equal(testString))
	})

	It("should stream standard error until it is stopped", func() {
		sid := str.Stream(stdoutChan, stderrChan)
		w := &syncBuffer{
			Buffer: new(bytes.Buffer),
		}
		go str.ServeStderr(sid, w)
		stderrChan <- testByteSlice
		stderrChan <- testByteSlice
		Eventually(w.String).Should(Equal("xx"))
		str.Stop(sid)
	})

	// The following test will not reliably fail if the implementation fails to drain messages.
	It("should stream the remaining standard error messages after being stopped", func() {
		sid := str.Stream(stdoutChan, stderrChan)
		str.Stop(sid)
		w := new(bytes.Buffer)
		stderrChan <- testByteSlice
		str.ServeStderr(sid, w)
		Consistently(w.String).Should(Equal(testString))
	})

	Context("when a grace time has been set", func() {
		BeforeEach(func() {
			graceTime = 100 * time.Millisecond

		})

		It("should not leak unused streams for longer than the grace time after streaming has been stopped", func() {
			sid := str.Stream(stdoutChan, stderrChan)
			str.Stop(sid)
			time.Sleep(200 * time.Millisecond)
			Expect(func() { str.Stop(sid) }).To(Panic(), "stream was not removed")
		})

		It("should not leak goroutines for longer than the grace time after streaming has been stopped", func() {
			initialNumGoroutine := runtime.NumGoroutine()

			sid := str.Stream(stdoutChan, stderrChan)
			str.Stop(sid)

			Eventually(func() int {
				return runtime.NumGoroutine()
			}, "200ms").Should(Equal(initialNumGoroutine))
		})
	})

	It("should terminate streaming output after a write error has occurred", func() {
		sid := str.Stream(stdoutChan, stderrChan)
		w := &syncBuffer{
			Buffer: new(bytes.Buffer),
			fail:   true,
		}
		go str.ServeStdout(sid, w)
		stdoutChan <- testByteSlice
		stdoutChan <- testByteSlice
		Consistently(w.String).Should(Equal(""))
		str.Stop(sid)
	})

	It("should terminate streaming errors after a write error has occurred", func() {
		sid := str.Stream(stdoutChan, stderrChan)
		w := &syncBuffer{
			Buffer: new(bytes.Buffer),
			fail:   true,
		}
		go str.ServeStderr(sid, w)
		stderrChan <- testByteSlice
		stderrChan <- testByteSlice
		Consistently(w.String).Should(Equal(""))
		str.Stop(sid)
	})
})

type syncBuffer struct {
	*bytes.Buffer
	fail bool
	mu   sync.Mutex
}

func (sb *syncBuffer) Write(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if sb.fail {
		sb.fail = false
		return 0, errors.New("failed")
	}
	return sb.Buffer.Write(p)
}

func (sb *syncBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.Buffer.String()
}
