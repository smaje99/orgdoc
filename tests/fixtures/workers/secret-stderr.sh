#!/bin/sh
printf 'Authorization: Bearer fixture-bearer-token\n' >&2
printf '{"api_key":"fixture-json-api-key","token": "fixture-json-token"}\n' >&2
exit 7
