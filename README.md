# RDS Backup Lambda

An AWS Lambda function that automates the process of creating, exporting, and managing Amazon RDS database backups across regions with S3 export capabilities.

## 🌟 Features

- **Cross-Region Backup**: Automatically create and copy RDS snapshots across AWS regions
- **S3 Export**: Export snapshots to S3 buckets in both source and target regions
- **Automated Cleanup**: Maintain snapshot hygiene with automatic cleanup of old snapshots
- **Email Notifications**: Receive success/failure notifications via Amazon SES
- **KMS Encryption**: Support for both single-region and multi-region KMS keys
- **Error Handling**: Robust error handling and reporting
- **Configurable**: Highly configurable through environment variables

## 📋 Prerequisites

- AWS Account with appropriate permissions
- AWS Lambda execution role with necessary permissions:
  - RDS: `rds:CreateDBSnapshot`, `rds:DeleteDBSnapshot`, `rds:DescribeDBSnapshots`, etc.
  - S3: `s3:PutObject`, `s3:GetObject`, etc.
  - KMS: `kms:Decrypt`, `kms:GenerateDataKey`, etc.
  - SES: `ses:SendEmail`, `ses:SendRawEmail`
  - IAM: Required permissions for cross-region operations
- Amazon SES configured and verified domain/email addresses
- S3 buckets in source and target regions
- KMS keys in both regions (or a multi-region key)

## ⚙️ Configuration

The lambda function is configured using environment variables:

```env
SOURCE_REGION=us-west-2
TARGET_REGION=us-east-1
DB_IDENTIFIER=my-database
SOURCE_BUCKET=source-backup-bucket
TARGET_BUCKET=target-backup-bucket
KMS_KEY_ID=mrk-1234567890abcdef  # Can be MRK or regular key ID
EXPORT_ROLE_ARN=arn:aws:iam::123456789012:role/rds-s3-export
ADMIN_EMAIL=admin@example.com
KEEP_SOURCE_SNAPSHOT=true
STORE_TO_SOURCE_S3=true
```

### Environment Variables Explained

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| SOURCE_REGION | AWS region where the source RDS instance is located | Yes | - |
| TARGET_REGION | AWS region where backups will be copied | Yes | - |
| DB_IDENTIFIER | RDS database identifier | Yes | - |
| SOURCE_BUCKET | S3 bucket in source region for exports | Yes | - |
| TARGET_BUCKET | S3 bucket in target region for exports | Yes | - |
| KMS_KEY_ID | KMS key ID for encryption (supports MRK) | Yes | - |
| EXPORT_ROLE_ARN | IAM role ARN for RDS to S3 export | Yes | - |
| ADMIN_EMAIL | Email address for notifications | Yes | - |
| KEEP_SOURCE_SNAPSHOT | Whether to retain source snapshot after copy | No | false |
| STORE_TO_SOURCE_S3 | Whether to store backup in source region S3 | No | false |

## 🚀 Deployment

1. Clone the repository:
```bash
git clone https://github.com/yourusername/rds-backup-lambda.git
cd rds-backup-lambda
```

2. Build the Lambda function:
```bash
GOOS=linux GOARCH=amd64 go build -o main cmd/backup/main.go
zip function.zip main
```

3. Create an AWS Lambda function:
- Runtime: Go 1.x
- Handler: main
- Memory: 256 MB (recommended)
- Timeout: 15 minutes (adjust based on your database size)

4. Deploy using AWS CLI:
```bash
aws lambda create-function \
  --function-name rds-backup-lambda \
  --runtime go1.x \
  --handler main \
  --zip-file fileb://function.zip \
  --role arn:aws:iam::123456789012:role/lambda-role
```

## 📝 IAM Role Permissions

Here's an example IAM policy for the Lambda execution role:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "rds:CreateDBSnapshot",
                "rds:DeleteDBSnapshot",
                "rds:DescribeDBSnapshots",
                "rds:CopyDBSnapshot",
                "rds:StartExportTask",
                "rds:DescribeExportTasks"
            ],
            "Resource": "*"
        },
        {
            "Effect": "Allow",
            "Action": [
                "s3:PutObject",
                "s3:GetObject",
                "s3:ListBucket"
            ],
            "Resource": [
                "arn:aws:s3:::source-bucket/*",
                "arn:aws:s3:::target-bucket/*"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "kms:Decrypt",
                "kms:GenerateDataKey"
            ],
            "Resource": [
                "arn:aws:kms:source-region:account-id:key/*",
                "arn:aws:kms:target-region:account-id:key/*"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "ses:SendEmail",
                "ses:SendRawEmail"
            ],
            "Resource": "*"
        }
    ]
}
```

## 📊 Monitoring

The lambda function logs all operations to CloudWatch Logs. You can monitor:
- Snapshot creation and copy progress
- Export task status
- Cleanup operations
- Error messages and stack traces

Recommended CloudWatch Metrics to monitor:
- Lambda execution duration
- Lambda errors
- Lambda throttles
- Lambda concurrent executions

## 📧 Email Notifications

The function sends HTML-formatted emails for both success and failure scenarios:

### Success Email
- Database identifier
- Snapshot ID
- Backup timestamp
- S3 locations (both regions)

### Failure Email
- Database identifier
- Attempted snapshot ID
- Error timestamp
- Detailed error message

## 🧹 Cleanup Process

The function automatically manages snapshot retention:
- Removes snapshots older than 15 days
- Cleans up in both source and target regions
- Maintains proper error logging
- Continues with remaining cleanup if one deletion fails

## 🛠️ Development

### Project Structure
```
rds-backup-lambda/
├── main.go                # Lambda entry point
├── internal/
│   ├── config/
│   │   └── config.go      # Configuration management
│   ├── backup/
│   │   ├── backup.go      # Core backup logic
│   │   └── snapshot.go    # Snapshot operations
│   ├── aws/
│   │   ├── kms.go         # KMS operations
│   │   └── session.go     # AWS session management
│   └── notification/
│       ├── email.go       # Email handling
│       └── templates.go   # Email templates
└── go.mod
```


