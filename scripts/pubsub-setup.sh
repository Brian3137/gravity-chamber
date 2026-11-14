#!/usr/bin/env bash
set -euo pipefail

: "${PUBSUB_EMULATOR_HOST:?set PUBSUB_EMULATOR_HOST}"

P="${PUBSUB_PROJECT_ID:-gravity-chamber}"
TOPIC="${1:-power-readings}"
SUB="${2:-ranker}"

curl -s -XPUT "http://$PUBSUB_EMULATOR_HOST/v1/projects/$P/topics/$TOPIC" >/dev/null

curl -s -XPUT "http://$PUBSUB_EMULATOR_HOST/v1/projects/$P/subscriptions/$SUB" \
  -H 'Content-Type: application/json' \
  -d "{\"topic\":\"projects/$P/topics/$TOPIC\",\"ackDeadlineSeconds\":10}" >/dev/null

echo "topic=$TOPIC sub=$SUB ready"