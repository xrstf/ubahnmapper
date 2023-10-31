#!/usr/bin/env bash

docker run \
  --rm \
  -it \
  -v "/dev:/dev" \
  -v "$(realpath .):/xanadu" \
  -w "/xanadu" \
  --privileged \
  ubahnmapper $@
