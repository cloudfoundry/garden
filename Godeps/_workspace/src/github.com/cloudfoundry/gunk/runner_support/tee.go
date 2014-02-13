package runner_support

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/onsi/ginkgo/config"
)

func TeeIfVerbose(out io.Writer) io.Writer {
	if config.DefaultReporterConfig.Verbose {
		return io.MultiWriter(out, os.Stdout)
	} else {
		return io.MultiWriter(out, ioutil.Discard)
	}
}
