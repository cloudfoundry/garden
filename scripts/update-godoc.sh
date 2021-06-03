#!/usr/bin/env bash
set -ex

cd garden
go run scripts/update-godoc/main.go
