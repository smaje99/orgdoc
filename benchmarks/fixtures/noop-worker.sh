#!/bin/sh
request=$(cat)
job_id=$(printf '%s\n' "$request" | sed -n 's/.*"job_id":"\([^"]*\)".*/\1/p')
printf '{"protocol_version":1,"job_id":"%s","status":"succeeded","outputs":[],"diagnostics":[]}\n' "$job_id"
