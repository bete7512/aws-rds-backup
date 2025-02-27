# AWS RDS Backup tool to other Region and export to S3

Built in Go for creating, exporting, and managing AWS RDS database backups across multiple regions.

## Overview

RDS Backup Manager automates the process of creating RDS snapshots, exporting them to S3, and optionally copying them to a secondary region for disaster recovery purposes. It runs as a standalone service with built-in scheduling.

## Features
- Runs every 24 hour
- Creates RDS manual snapshots on a scheduled basis
- Exports snapshots to S3 in both source and target regions
- Supports cross-region replication of snapshots
- Automatic cleanup of old snapshots (45 days retention)
- Email notifications for successful and failed backups
- Configurable via environment variables
- Graceful shutdown handling

# Prerequisites

- AWS Account with appropriate permissions
- An RDS database instance
- S3 buckets in source and target regions
- KMS key for encryption(Multi Region KMS Key)
- IAM role with permissions for RDS snapshot export

## Required IAM Permissions

The application requires IAM permissions for:

- RDS: CreateDBSnapshot, DescribeDBSnapshots, DeleteDBSnapshot, CopyDBSnapshot
- RDS: StartExportTask, DescribeExportTasks
- S3: PutObject on both source and target buckets
- KMS: Encrypt, Decrypt permissions on the specified KMS key
- SES: SendEmail (for notifications(but can be updated to other email sender))

## Installation

### Clone the repository

```bash
git clone https://github.com/unplank/rds-backup-lambda.git
cd rds-backup-lambda
```

### Build the application

```bash
go build -o rds-backup-manager
```

## Configuration

Create a `.env` file in the project root with the following variables:

```
SOURCE_REGION=us-east-1        # Region where your DB instance is located
TARGET_REGION=us-west-2        # DR region for snapshot copy
DB_IDENTIFIER=my-database      # RDS database identifier
SOURCE_BUCKET=source-backups   # S3 bucket in source region
TARGET_BUCKET=target-backups   # S3 bucket in target region
KMS_KEY_ID=mrk-abcd1234        # KMS key ID (should be MRK for multi-region)
EXPORT_ROLE_ARN=arn:aws:iam::123456789012:role/rds-export-role  # IAM role ARN for RDS export
ADMIN_EMAIL=admin@example.com  # Email for notifications
ADMIN_EMAILS=admin1@example.com,admin2@example.com  # Comma-separated list of notification recipients
KEEP_SOURCE_SNAPSHOT=true      # Whether to keep the source snapshot after copying to target region
STORE_TO_SOURCE_S3=true        # Whether to export snapshot to source S3 bucket
```

## Running the Application

```bash
./rds-backup-manager
```

The application will:
1. Run an initial backup immediately
2. Schedule daily backups at midnight UTC
3. Send email notifications for backup results

## Backup Process

1. Creates a snapshot of the specified RDS instance
2. Waits for the snapshot to become available
3. Exports the snapshot to S3 in the source region (if configured)
4. Copies the snapshot to the target region
5. Exports the copied snapshot to S3 in the target region
6. Cleans up snapshots older than 45 days
7. Optionally deletes the source snapshot if not needed

## Handling Database States

The application intelligently handles various database states:
- If the database is in "backing-up" state, it checks for existing snapshots from today
- If the database is in "available" state, it creates a new snapshot
- For other states, it retries periodically (up to 1 hour)