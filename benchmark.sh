#!/bin/bash

set -euo pipefail

DIFFICULTIES=(1 5 10 15)
FETCH_WORKERS=4
TIMEOUT=1000
CLIENT_RUNS=5

printf "%-12s %-10s %-10s %-10s %-10s\n" "Difficulty" "AvgTime(s)" "Ops/sec" "MaxRSS(MB)" "CPU(s)"
RESULTS=()

for DIFFICULTY in "${DIFFICULTIES[@]}"; do
  export DIFFICULTY
  export FETCH_WORKERS
  export TIMEOUT

  # Port cleanup with confirmation
  if lsof -i :9000 -sTCP:LISTEN -t >/dev/null ; then
    echo "Port 9000 is in use. The following process(es) are using it:"
    lsof -i :9000 -sTCP:LISTEN
    read -p "Do you want to kill these process(es) to continue? [y/N]: " confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
      fuser -k 9000/tcp 2>/dev/null || true
      echo "Killed process(es) using port 9000."
    else
      echo "Aborting benchmark for difficulty $DIFFICULTY."
      continue
    fi
  fi

  cd cmd/server
  go run . &
  SERVER_PID=$!
  cd ../..

  # Wait for server
  for i in {1..10}; do
    if curl -s http://localhost:8080/healthz | grep -q ok; then
      break
    fi
    sleep 1
  done

  total_time=0
  total_ops=0
  max_rss=0
  total_cpu=0

  for run in $(seq 1 $CLIENT_RUNS); do
    cd cmd/client
    # Use /usr/bin/time for resource usage
    TIMEFMT=$'real %e\nuser %U\nsys %S\nmaxrss %M'
    { time_output=$( (/usr/bin/time -f "$TIMEFMT" go run .) 2>&1 1>/dev/null ); } || true
    cd ../..

    real_time=$(echo "$time_output" | awk '/real/ {print $2}')
    user_time=$(echo "$time_output" | awk '/user/ {print $2}')
    sys_time=$(echo "$time_output" | awk '/sys/ {print $2}')
    rss_kb=$(echo "$time_output" | awk '/maxrss/ {print $2}')
    rss_mb=$((rss_kb/1024))
    cpu_time=$(echo "$user_time + $sys_time" | bc)

    # Fetch pow_challenge_served_total
    pow_challenges=$(curl -s http://localhost:8080/metrics | grep '^pow_challenge_served_total ' | awk '{print $2}')
    pow_challenges=${pow_challenges:-0}

    ops_per_sec=$(awk -v ops="$pow_challenges" -v t="$real_time" 'BEGIN { if (t>0) print ops/t; else print 0 }')

    total_time=$(awk -v t="$total_time" -v r="$real_time" 'BEGIN {print t+r}')
    total_ops=$(awk -v o="$total_ops" -v p="$pow_challenges" 'BEGIN {print o+p}')
    total_cpu=$(awk -v c="$total_cpu" -v n="$cpu_time" 'BEGIN {print c+n}')
    if (( rss_mb > max_rss )); then max_rss=$rss_mb; fi
  done

  avg_time=$(awk -v t="$total_time" -v n="$CLIENT_RUNS" 'BEGIN {if(n>0) print t/n; else print 0}')
  avg_ops=$(awk -v o="$total_ops" -v t="$total_time" 'BEGIN {if(t>0) print o/t; else print 0}')
  avg_cpu=$(awk -v c="$total_cpu" -v n="$CLIENT_RUNS" 'BEGIN {if(n>0) print c/n; else print 0}')

  printf "%-12s %-10s %-10s %-10s %-10s\n" "$DIFFICULTY" "$avg_time" "$avg_ops" "$max_rss" "$avg_cpu"
  RESULTS+=("$DIFFICULTY $avg_time $avg_ops $max_rss $avg_cpu")

  kill $SERVER_PID
  wait $SERVER_PID 2>/dev/null || true
  sleep 1

done

echo -e "\nSummary Table:"
printf "%-12s %-10s %-10s %-10s %-10s\n" "Difficulty" "AvgTime(s)" "Ops/sec" "MaxRSS(MB)" "CPU(s)"
for row in "${RESULTS[@]}"; do
  printf "%-12s %-10s %-10s %-10s %-10s\n" $row
 done 