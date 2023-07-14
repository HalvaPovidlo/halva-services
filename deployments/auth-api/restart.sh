#!/bin/bash

cd $(dirname "$0")
export CONFIG_PATH=./secret.yaml

killall halva-login
echo "login restarted"
nohup ./halva-login 1>login.log 2>&1 &

echo "script finished"
