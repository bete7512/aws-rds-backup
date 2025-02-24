#!/bin/bash

# Create logs directory if it doesn't exist
mkdir -p logs

# Build the Go application
echo "Building RDS backup service..."
go build -o rds-backup-service

if [ $? -ne 0 ]; then
  echo "Build failed. Please check for errors."
  exit 1
fi

echo "Build successful!"

# Check if service is already running
PID=$(pgrep -f "rds-backup-service")
if [ ! -z "$PID" ]; then
  echo "Service is already running with PID $PID"
  echo "Stopping existing service..."
  kill -15 $PID
  sleep 2
fi

# Run the application in the background with nohup
echo "Starting RDS backup service in background..."
nohup ./rds-backup-service > logs/rds-backup.log 2>&1 &

# Get the PID of the new process
NEW_PID=$!
echo "Service started with PID $NEW_PID"

# Create a PID file for later reference
echo $NEW_PID > rds-backup-service.pid

echo "Service is now running in the background."
echo "Logs are being written to logs/rds-backup.log"
echo "To stop the service: kill -15 \$(cat rds-backup-service.pid)"
echo "To view logs: tail -f logs/rds-backup.log"