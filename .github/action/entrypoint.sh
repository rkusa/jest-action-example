#!/bin/sh

sh -c "$JEST_CMD $* --ci --testLocationInResults --json --outputFile=/tmp/report.json" &> /dev/null
set -e
sh -c "cat /tmp/report.json | /usr/bin/jest-action"