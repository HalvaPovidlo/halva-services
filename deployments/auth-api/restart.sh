#!/bin/bash

cd $(dirname "$0")
export CONFIG_PATH=./secret.yaml

killall halva-login

cp login.log "logs/login_$(date +%F).log"
nohup ./halva-login 1>login.log 2>&1 &

echo "script finished"
