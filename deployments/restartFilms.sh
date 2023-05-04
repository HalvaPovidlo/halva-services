#!/bin/bash

cd $(dirname "$0")

killall halva-films
echo "films restarted"
nohup ./halva-films 1>films.log 2>&1 &

echo "script finished"
