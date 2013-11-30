// +build linux

package main_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vito/cmdtest"
	. "github.com/vito/cmdtest/matchers"
)

var _ = Describe("Running wshd", func() {
	wshd, err := cmdtest.Build("github.com/vito/garden/backend/linux_backend/wshd")
	if err != nil {
		panic(err)
	}

	wsh, err := cmdtest.Build("github.com/vito/garden/backend/linux_backend/wshd/wsh")
	if err != nil {
		panic(err)
	}

	var socketPath string
	var containerPath string
	var wshdCommand *exec.Cmd

	BeforeEach(func() {
		containerDir, err := ioutil.TempDir(os.TempDir(), "wshd-test-container")
		Expect(err).ToNot(HaveOccured())

		containerPath = containerDir

		binDir := path.Join(containerDir, "bin")
		libDir := path.Join(containerDir, "lib")
		runDir := path.Join(containerDir, "run")
		mntDir := path.Join(containerDir, "mnt")

		os.Mkdir(binDir, 0755)
		os.Mkdir(libDir, 0755)
		os.Mkdir(runDir, 0755)

		err = copyFile(wshd, path.Join(binDir, "wshd"))
		Expect(err).ToNot(HaveOccured())

		ioutil.WriteFile(path.Join(libDir, "hook-parent-before-clone.sh"), []byte(`#!/bin/bash

set -o xtrace
set -o nounset
set -o errexit
shopt -s nullglob

cd $(dirname $0)/../

cp bin/wshd mnt/sbin/wshd
chmod 700 mnt/sbin/wshd
`), 0755)

		ioutil.WriteFile(path.Join(libDir, "hook-parent-after-clone.sh"), []byte(`#!/bin/bash

set -o xtrace
set -o nounset
set -o errexit
shopt -s nullglob

cd $(dirname $0)/../

echo $PID > ./run/wshd.pid
`), 0755)

		ioutil.WriteFile(path.Join(libDir, "hook-child-before-pivot.sh"), []byte(`#!/bin/bash
env
pwd
`), 0755)

		ioutil.WriteFile(path.Join(libDir, "hook-child-after-pivot.sh"), []byte(`#!/bin/bash

set -o xtrace
set -o nounset
set -o errexit
shopt -s nullglob

cd $(dirname $0)/../

mkdir -p /dev/pts
mount -t devpts -o newinstance,ptmxmode=0666 devpts /dev/pts
ln -sf pts/ptmx /dev/ptmx

mkdir -p /proc
mount -t proc none /proc

`), 0755)

		ioutil.WriteFile(path.Join(libDir, "set-up-root.sh"), []byte(`#!/bin/bash

set -o xtrace
set -o nounset
set -o errexit
shopt -s nullglob

rootfs_path=$1

function get_distrib_codename() {
  if [ -r /etc/lsb-release ]
  then
    source /etc/lsb-release

    if [ -n "$DISTRIB_CODENAME" ]
    then
      echo $DISTRIB_CODENAME
      return 0
    fi
  else
    lsb_release -cs
  fi
}

function overlay_directory_in_rootfs() {
  # Skip if exists
  if [ ! -d tmp/rootfs/$1 ]
  then
    if [ -d mnt/$1 ]
    then
      cp -r mnt/$1 tmp/rootfs/
    else
      mkdir -p tmp/rootfs/$1
    fi
  fi

  mount -n --bind tmp/rootfs/$1 mnt/$1
  mount -n --bind -o remount,$2 tmp/rootfs/$1 mnt/$1
}

function setup_fs_other() {
  mkdir -p tmp/rootfs mnt
  mkdir -p $rootfs_path/proc

  mount -n --bind $rootfs_path mnt
  mount -n --bind -o remount,ro $rootfs_path mnt

  overlay_directory_in_rootfs /dev rw
  overlay_directory_in_rootfs /etc rw
  overlay_directory_in_rootfs /home rw
  overlay_directory_in_rootfs /sbin rw
  overlay_directory_in_rootfs /var rw

  mkdir -p tmp/rootfs/tmp
  chmod 777 tmp/rootfs/tmp
  overlay_directory_in_rootfs /tmp rw
}

function setup_fs_ubuntu() {
  mkdir -p tmp/rootfs mnt

  distrib_codename=$(get_distrib_codename)

  case "$distrib_codename" in
  lucid|natty|oneiric)
    mount -n -t aufs -o br:tmp/rootfs=rw:$rootfs_path=ro+wh none mnt
    ;;
  precise)
    mount -n -t overlayfs -o rw,upperdir=tmp/rootfs,lowerdir=$rootfs_path none mnt
    ;;
  *)
    echo "Unsupported: $distrib_codename"
    exit 1
    ;;
  esac
}

function setup_fs() {
  if grep -q -i ubuntu /etc/issue
  then
    setup_fs_ubuntu
  else
    setup_fs_other
  fi
}

setup_fs
`), 0755)

		setUpRoot := exec.Command(path.Join(libDir, "set-up-root.sh"), os.Getenv("GARDEN_TEST_ROOTFS"))
		setUpRoot.Dir = containerDir

		setUpRootSession, err := cmdtest.StartWrapped(setUpRoot, outWrapper, outWrapper)
		Expect(err).ToNot(HaveOccured())
		Expect(setUpRootSession).To(ExitWith(0))

		socketPath = path.Join(runDir, "wshd.sock")

		wshdCommand = exec.Command(wshd, "--run", runDir, "--lib", libDir, "--root", mntDir)

		wshdSession, err := cmdtest.StartWrapped(wshdCommand, outWrapper, outWrapper)
		Expect(err).ToNot(HaveOccured())

		Expect(wshdSession).To(ExitWith(0))

		createdContainers = append(createdContainers, containerDir)

		Eventually(ErrorDialingUnix(socketPath)).ShouldNot(HaveOccured())
	})

	It("starts the daemon with process isolation", func() {
		ps := exec.Command(wsh, "--socket", socketPath, "/bin/ps", "-o", "pid,command")

		psSession, err := cmdtest.StartWrapped(ps, outWrapper, outWrapper)
		Expect(err).ToNot(HaveOccured())

		Expect(psSession).To(Say(" 1 /sbin/wshd"))
		Expect(psSession).To(ExitWith(0))
	})

	PIt("starts the daemon with mount space isolation", func() {
	})

	PIt("starts the daemon with network namespace isolation", func() {

	})

	PIt("starts the daemon with a new IPC namespace", func() {

	})

	PIt("starts the daemon with a new UTCS namespace", func() {

	})

	PIt("makes the given rootfs the root of the daemon", func() {

	})

	PIt("executes the hooks in the proper sequence", func() {

	})

	PIt("does not leak file descriptors to the child", func() {
		wshdPidfile, err := os.Open(path.Join(containerPath, "run", "wshd.pid"))
		Expect(err).ToNot(HaveOccured())

		var wshdPid int

		_, err = fmt.Fscanf(wshdPidfile, "%d", &wshdPid)
		Expect(err).ToNot(HaveOccured())

		entries, err := ioutil.ReadDir(fmt.Sprintf("/proc/%d/fd", wshdPid))
		Expect(err).ToNot(HaveOccured())

		// TODO: one fd is wshd.sock, unsure what the other is.
		// shows up as type 0000 in lsof.
		//
		// compare with original wshd
		Expect(entries).To(HaveLen(2))
	})

	It("unmounts /mnt* in the child", func() {
		cat := exec.Command(wsh, "--socket", socketPath, "/bin/cat", "/proc/mounts")

		catSession, err := cmdtest.StartWrapped(cat, outWrapper, outWrapper)
		Expect(err).ToNot(HaveOccured())

		Expect(catSession).ToNot(Say(" /mnt"))
		Expect(catSession).To(ExitWith(0))
	})
})

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}

	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = io.Copy(d, s)
	if err != nil {
		d.Close()
		return err
	}

	return d.Close()
}

func outWrapper(out io.Reader) io.Reader {
	return io.TeeReader(out, os.Stdout)
}

func ErrorDialingUnix(socketPath string) func() error {
	return func() error {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
		}

		return err
	}
}
