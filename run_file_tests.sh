#!/bin/bash

# Load environment variables from .env file
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

# Run the file-based tests
go test -v -run TestCacheInjectionFileBasedScenarios -timeout 10m