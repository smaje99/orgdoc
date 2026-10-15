#!/bin/sh
printf 'worker artifact\n' > result.txt
printf '{"protocol_version":1,"job_id":"job-1","status":"succeeded","outputs":[{"path":"result.txt","media_type":"text/plain"}],"diagnostics":[]}\n'
