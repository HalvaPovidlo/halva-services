#!/bin/bash

cd $(dirname "$0")
export CONFIG_PATH=./secret.yaml

killall halva-films

cp films.log "logs/films_$(date +%F).log"
nohup ./halva-films 1>films.log 2>&1 &

echo "script finished"
