package connection

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"code.cloudfoundry.org/lager/v3/lagertest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("streamHandler (Internal)", func() {
	var sh *streamHandler

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("stream-handler-internal-test")
		sh = newStreamHandler(logger)
	})

	Context("when streamOut is called concurrently on the same handler instance", func() {
		It("serializes writes to a shared writer, preventing data corruption", func() {
			sharedWriter := new(bytes.Buffer)

			var testWg sync.WaitGroup
			numGoroutines := 120
			dataChunk := "abcdefghijklmnopqrstuvwxyz"

			for i := 0; i < numGoroutines; i++ {
				testWg.Add(1)
				go func(id int) {
					defer GinkgoRecover()
					defer testWg.Done()

					uniqueData := fmt.Sprintf("[%d:%s]", id, dataChunk)
					reader := strings.NewReader(uniqueData)

					sh.streamOut(sharedWriter, reader)

					time.Sleep(time.Millisecond)
				}(i)
			}

			testWg.Wait()
			sh.wg.Wait()

			finalOutput := sharedWriter.String()
			for i := 0; i < numGoroutines; i++ {
				expectedChunk := fmt.Sprintf("[%d:%s]", i, dataChunk)
				Expect(finalOutput).To(ContainSubstring(expectedChunk),
					fmt.Sprintf("Final output should contain the complete, uncorrupted data chunk for goroutine %d", i))
			}

			var expectedLength int
			for i := 0; i < numGoroutines; i++ {
				expectedLength += len(fmt.Sprintf("[%d:%s]", i, dataChunk))
			}

			Expect(len(finalOutput)).To(Equal(expectedLength), "Final output should have the combined length of all chunks")
		})
	})

	Context("when both streamOut and streamErr are called", func() {
		It("streams messages to both, without delaying either until the other finishes", func() {
			stdout := gbytes.NewBuffer()
			stderr := gbytes.NewBuffer()

			toStreamOutReader, toStreamOutWriter := io.Pipe()
			toStreamErrReader, toStreamErrWriter := io.Pipe()

			sh.streamOut(stdout, toStreamOutReader)
			sh.streamErr(stderr, toStreamErrReader)

			go toStreamOutWriter.Write([]byte("this is stdout"))
			go toStreamErrWriter.Write([]byte("this is stderr"))

			Eventually(stdout).Should(gbytes.Say("this is stdout"))
			Eventually(stderr).Should(gbytes.Say("this is stderr"))

			toStreamOutWriter.Close()
			toStreamErrWriter.Close()
		})
	})
})
