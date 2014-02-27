package cmdtest_matchers

import (
	"fmt"
	"time"

	"github.com/vito/cmdtest"
)

func ExitWithTimeout(status int, timeout time.Duration) *ExitWithTimeoutMatcher {
	return &ExitWithTimeoutMatcher{status, timeout}
}

type ExitWithTimeoutMatcher struct {
	Status int
	Timeout time.Duration
}

func (m *ExitWithTimeoutMatcher) Match(out interface{}) (bool, string, error) {
	session, ok := out.(*cmdtest.Session)
	if !ok {
		return false, "", fmt.Errorf("Cannot expect exit status from %#v.", out)
	}

	status, err := session.Wait(m.Timeout)
	if err != nil {
		return false, err.Error(), nil
	}

	if status == m.Status {
		return true, fmt.Sprintf("Expected to not exit with %#v", m.Status), nil
	} else {
		return false, fmt.Sprintf("Exited with status %d, expected %d", status, m.Status), nil
	}
}
