#!/bin/bash

cd $(dirname "$0")
export CONFIG_PATH=./secret.yaml

killall halva-films
echo "films restarted"
nohup ./halva-films 1>films.log 2>&1 &

echo "script finished"
