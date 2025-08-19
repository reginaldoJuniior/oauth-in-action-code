#!/bin/bash

# Set default ports if not provided
CLIENT_PORT=${CLIENT_PORT:-9000}
AUTH_PORT=${AUTH_PORT:-9001}
RESOURCE_PORT=${RESOURCE_PORT:-9002}

# Run all three servers in background
CLIENT_PORT=$CLIENT_PORT go run client.go &
AUTH_PORT=$AUTH_PORT go run authorizationServer.go &
RESOURCE_PORT=$RESOURCE_PORT go run protectedResource.go &

# Wait for all to finish (Ctrl+C to stop)
wait
