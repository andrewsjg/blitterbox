#!/bin/bash
cd "$(dirname "$0")"
BLITTERBOX_DIAG=1 go test -count=1 -v -run TestBootDecision -timeout 120s -tags noebitengine ./amiga/ 2>&1
