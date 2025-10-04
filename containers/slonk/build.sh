#!/bin/bash
echo "" > extra.sh
docker build "$@" .
rm extra.sh
