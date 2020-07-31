#!/usr/bin/env bash

BASEDIR=$(dirname "$0")
cd "${BASEDIR}/.." || exit 1

rm -rf out
mkdir out

ID=$(docker build -q .)

docker run --rm -v "$(pwd)/out:/out-out" "$ID" cp /out/docker-utils /out-out/docker-utils

docker image rm "$ID"