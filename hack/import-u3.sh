#!/usr/bin/env bash

set -e

cd $(dirname $0)/..

set -x
go build -v ./cmd/importer
./importer -p data/u3-runde-1.txt -r -i u3-loop1-collapsed --collapse-stops 2m data/u3-runde-1.csv > u3-runde-1-collapsed.sql
./importer -p data/u3-runde-2.txt -s 1h -r -i u3-loop2-collapsed --collapse-stops 2m data/u3-runde-2.csv > u3-runde-2-collapsed.sql
