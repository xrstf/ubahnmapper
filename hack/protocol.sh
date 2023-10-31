#!/usr/bin/env bash

set -euo pipefail

cd $(dirname $0)/..

now=$(date +'%Y-%m-%dT%H:%M:%S')

echo "$now;$@" | tee -a protocol.txt
