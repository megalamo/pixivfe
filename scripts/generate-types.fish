#!/usr/bin/fish

opengrep scan --quiet -f i18n/opengrep.yaml --json | jq .results | quicktype -l python --just-types > i18n/extract/types.py
