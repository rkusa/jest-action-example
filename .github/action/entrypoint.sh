#!/bin/sh

env
sh -c "$JEST_CMD $* --ci --testLocationInResults --json --outputFile=report.json"
set -e
sh -c "cat report.json | /usr/bin/jest-action"