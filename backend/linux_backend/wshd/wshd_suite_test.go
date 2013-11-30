package main_test

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var createdContainers = []string{}

func TestWshd(t *testing.T) {
	if os.Getenv("GARDEN_TEST_ROOTFS") != "" {
		RegisterFailHandler(Fail)

		RunSpecs(t, "wshd Suite")

		for _, containerDir := range createdContainers {
			log.Println("cleaning up", containerDir)

			wshdPidfile, err := os.Open(path.Join(containerDir, "run", "wshd.pid"))
			if err == nil {
				var wshdPid int

				_, err := fmt.Fscanf(wshdPidfile, "%d", &wshdPid)
				if err == nil {
					proc, err := os.FindProcess(wshdPid)
					if err == nil {
						log.Println("killing", wshdPid, proc.Kill())
					}
				}
			}

			for i := 0; i < 100; i++ {
				umount := exec.Command("umount", path.Join(containerDir, "mnt"))

				err := umount.Run()

				log.Println("unmounting", err)

				if err == nil {
					break
				}

				time.Sleep(1 * time.Second)
			}
		}

		for _, containerDir := range createdContainers {
			for i := 0; i < 100; i++ {
				err := os.RemoveAll(containerDir)

				log.Println("destroying", err)

				if err == nil {
					break
				}

				time.Sleep(1 * time.Second)
			}
		}
	}
}
