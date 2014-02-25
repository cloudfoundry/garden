#!/bin/bash

[ -n "$DEBUG" ] && set -o xtrace
set -o nounset
set -o errexit
shopt -s nullglob

cd $(dirname "${0}")

# Check if the old mount point exists, and if so clean it up
if [ -d /dev/cgroup ]
then
  if grep -q /dev/cgroup /proc/mounts
  then
    umount /dev/cgroup
  fi

  rmdir /dev/cgroup
fi

cgroup_path=/tmp/warden/cgroup

function mount_flat_cgroup() {
  cgroup_parent_path=$(dirname $1)

  mkdir -p $cgroup_parent_path

  if ! grep "${cgroup_parent_path} " /proc/mounts | cut -d' ' -f3 | grep -q tmpfs
  then
    mount -t tmpfs none $cgroup_parent_path
  fi

  mkdir -p $1
  mount -t cgroup none $1

  # bind-mount cgroup subsystems to make file tree consistent
  for subsystem in cpu cpuacct cpuset devices memory
  do
    mkdir -p ${1}/$subsystem

    if ! grep -q "${1}/$subsystem " /proc/mounts
    then
      mount --bind $1 ${1}/$subsystem
    fi
  done
}

function mount_nested_cgroup() {
  mkdir -p $1

  if ! grep "${cgroup_path} " /proc/mounts | cut -d' ' -f3 | grep -q tmpfs
  then
    mount -t tmpfs none $1
  fi

  for subsystem in cpu cpuacct cpuset devices memory
  do
    mkdir -p ${1}/$subsystem

    if ! grep -q "${1}/$subsystem " /proc/mounts
    then
      mount -t cgroup -o $subsystem none ${1}/$subsystem
    fi
  done
}

if [ ! -d $cgroup_path ]
then
  # temporarily mount a flat cgroup just to see if we can
  cgroup_check_path=/tmp/warden_cgroup_check

  mkdir -p $cgroup_check_path

  if mount -t cgroup none $cgroup_check_path; then
    umount $cgroup_check_path
    rmdir $cgroup_check_path
    mount_flat_cgroup $cgroup_path
  else
    rmdir $cgroup_check_path
    mount_nested_cgroup $cgroup_path
  fi
fi

./net.sh setup

# Disable AppArmor if possible
if [ -x /etc/init.d/apparmor ]; then
  /etc/init.d/apparmor teardown
fi

# quotaon(8) exits with non-zero status when quotas are ENABLED
if [ "$DISK_QUOTA_ENABLED" = "true" ] && quotaon -p $CONTAINER_DEPOT_MOUNT_POINT_PATH > /dev/null 2>&1
then
  mount -o remount,usrjquota=aquota.user,grpjquota=aquota.group,jqfmt=vfsv0 $CONTAINER_DEPOT_MOUNT_POINT_PATH
  quotacheck -ugmb -F vfsv0 $CONTAINER_DEPOT_MOUNT_POINT_PATH
  quotaon $CONTAINER_DEPOT_MOUNT_POINT_PATH
elif [ "$DISK_QUOTA_ENABLED" = "false" ] && ! quotaon -p $CONTAINER_DEPOT_MOUNT_POINT_PATH > /dev/null 2>&1
then
  quotaoff $CONTAINER_DEPOT_MOUNT_POINT_PATH
fi
