#!/bin/sh
(sleep 0.25; printf 'child survived\n' > child-survived) &
printf 'ready\n' > parent-ready
while :; do sleep 1; done
