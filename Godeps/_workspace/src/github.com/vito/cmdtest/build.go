package cmdtest

import (
	"io/ioutil"
	"os"
	"os/exec"
)

func Build(mainPath string) (string, error) {
	return BuildIn(mainPath, os.Getenv("GOPATH"))
}

func BuildIn(mainPath string, gopath string) (string, error) {
	if len(gopath) == 0 {
		panic("$GOPATH not provided when building " + mainPath)
	}

	executable, err := ioutil.TempFile(os.TempDir(), "test_cmd_main")
	if err != nil {
		return "", err
	}

	err = os.Remove(executable.Name())
	if err != nil {
		return "", err
	}

	build := exec.Command("go", "build", "-o", executable.Name(), mainPath)
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	build.Stdin = os.Stdin
	build.Env = []string{"GOPATH=" + gopath}

	err = build.Run()
	if err != nil {
		return "", err
	}

	return executable.Name(), nil
}
