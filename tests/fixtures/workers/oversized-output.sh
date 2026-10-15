#!/bin/sh
awk 'BEGIN { for (i = 0; i < 4096; i++) printf "x" }'
