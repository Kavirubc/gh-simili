#!/bin/bash
set -e
go build -o "dist/$1" ./cmd/gh-simili
