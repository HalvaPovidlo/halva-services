#!/bin/bash

cd $(dirname "$0")
export CONFIG_PATH=./secret.yaml

killall halva-host
echo "host restarted"
nohup ./halva-host 1>host.log 2>&1 &

echo "host finished"
