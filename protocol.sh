#!/usr/bin/env bash

now=$(date +'%Y-%m-%dT%H:%M:%S')

echo "$now;$@" | tee -a protocol.txt
