#!/bin/bash

# Check if service is running
if [ -f rds-backup-service.pid ]; then
  PID=$(cat rds-backup-service.pid)
  if ps -p $PID > /dev/null; then
    echo "RDS backup service is running with PID $PID"
    
    # Show recent logs
    echo ""
    echo "Last 10 log entries:"
    echo "--------------------"
    tail -n 10 logs/rds-backup.log
  else
    echo "PID file exists but service is not running. It may have crashed."
    echo "Check logs for details: logs/rds-backup.log"
  fi
else
  echo "RDS backup service is not running."
fi