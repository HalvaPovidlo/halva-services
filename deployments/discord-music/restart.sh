#!/bin/bash

cd $(dirname "$0")
export CONFIG_PATH=./secret.yaml

killall halva-music

./yt-dlp -U
cp music.log "logs/music_$(date +%F).log"
nohup ./halva-music 1>music.log 2>&1 &

echo "script finished"
