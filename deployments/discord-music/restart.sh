#!/bin/bash

cd $(dirname "$0")
export CONFIG_PATH=./secret.yaml

killall halva-music
echo "music restarted"
./yt-dlp -U
nohup ./halva-music 1>music.log 2>&1 &

echo "script finished"
