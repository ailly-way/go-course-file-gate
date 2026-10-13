#!/bin/sh
set -eu

deadline=$(date -u -v+30M '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || date -u -d '+30 minutes' '+%Y-%m-%dT%H:%M:%SZ')
curl --fail-with-body --request POST http://localhost:8080/downloads \
  --header 'Content-Type: application/json' \
  --data "{\"session_id\":\"$SESSION_ID\",\"learner_id\":\"learner-42\",\"course_id\":\"go-101\",\"object_key\":\"go-101/module-2.pdf\",\"deadline\":\"$deadline\"}"
