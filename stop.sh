#!/bin/bash

# Check if service is running
if [ -f rds-backup-service.pid ]; then
  PID=$(cat rds-backup-service.pid)
  if ps -p $PID > /dev/null; then
    echo "Stopping RDS backup service (PID: $PID)..."
    kill -15 $PID
    
    # Wait for process to terminate
    for i in {1..10}; do
      if ! ps -p $PID > /dev/null; then
        echo "Service stopped successfully."
        rm rds-backup-service.pid
        exit 0
      fi
      echo "Waiting for service to stop... ($i/10)"
      sleep 1
    done
    
    echo "Service did not stop gracefully. Forcing termination..."
    kill -9 $PID
    rm rds-backup-service.pid
    echo "Service terminated."
  else
    echo "PID file exists but service is not running."
    rm rds-backup-service.pid
  fi
else
  echo "RDS backup service is not running."
fi