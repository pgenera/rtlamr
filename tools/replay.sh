#!/bin/bash
# usage: replay.sh RTLAMR FILE SYMBOLLENGTH RATE OUTPREFIX [SPEED]
# Replays FILE (rtl_sdr .cu8 at RATE S/s) into the RTLAMR binary as fast as it
# can decode (or SPEED x real time); writes OUTPREFIX.json/.err/.fake/.cpu.
# Set RTLAMR_SAMPLECLOCK=1 (replay-tools build) to stamp messages with stream
# time, then: python3 tools/gaps.py OUTPREFIX.json DURATION_S
BIN=$1; F=$2; SL=$3; RATE=$4; O=$5; SPEED=${6:-0}
T=$(dirname "$0")
PORT=$((20000 + RANDOM % 20000))
FAKE=$(mktemp -d)/fakertltcp
(cd "$T/fakertltcp" && go build -o "$FAKE" .) || exit 1
"$FAKE" -addr 127.0.0.1:$PORT -file $F -rate $RATE -speed $SPEED > $O.fake &
sleep 0.5
$BIN -server=127.0.0.1:$PORT -symbollength=$SL -centerfreq=912380000 -msgtype=scm,scm+,r900 -unique=false -format=json > $O.json 2> $O.err &
P=$!; last=0
while kill -0 $P 2>/dev/null; do
  t=$(awk '{print $14+$15}' /proc/$P/stat 2>/dev/null) && last=$t
  sleep 0.5
done
wait
rec=$(awk '/^end/{sub("recorded=","",$4); sub("s","",$4); print $4}' $O.fake)
echo "cpu_s=$(echo "scale=2; $last/100" | bc) recorded_s=$rec cpu_pct_of_realtime=$(echo "scale=2; $last/$rec" | bc)%" > $O.cpu
cat $O.cpu
