# 🚀 Deployment Guide for RDS Backup Lambda

This guide provides detailed instructions for building, deploying, and scheduling the RDS Backup Lambda function.

## Table of Contents
- [Prerequisites](#prerequisites)
- [Building the Lambda Function](#building-the-lambda-function)
- [Deployment Process](#deployment-process)
- [Setting up CloudWatch Events](#setting-up-cloudwatch-events)
- [Testing the Lambda Function](#testing-the-lambda-function)
- [Updating the Lambda Function](#updating-the-lambda-function)
- [Troubleshooting](#troubleshooting)

## Prerequisites

Before starting the deployment process, ensure you have:

1. AWS CLI installed and configured with appropriate credentials
2. Go 1.x installed on your development machine
3. Necessary AWS permissions to:
   - Create/update Lambda functions
   - Access S3 buckets
   - Create CloudWatch Events rules
   - Configure IAM roles
4. S3 bucket created for storing Lambda deployment packages

## Building the Lambda Function

### Local Build Process

```bash
# Set environment variables for Linux build
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

# Build the binary
go build -a -installsuffix cgo -o bootstrap

# Create deployment package
zip function.zip bootstrap
```

### Build Script
Create a `build.sh` file:

```bash
#!/bin/bash
set -e

echo "Building Lambda function..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o bootstrap
echo "Creating deployment package..."
zip function.zip bootstrap
echo "Build complete!"
```

Make it executable:
```bash
chmod +x build.sh
```

## Deployment Process

### 1. Upload to S3

```bash
# Upload deployment package to S3
aws s3 cp function.zip s3://rds-backup-stack-lambda/

# Verify upload
aws s3 ls s3://rds-backup-stack-lambda/function.zip
```

### 2. Update Lambda Function

```bash
# Update Lambda function code
aws lambda update-function-code \
    --function-name rds-backup-stack-rds-backup \
    --s3-bucket rds-backup-stack-lambda \
    --s3-key function.zip

# Update Lambda configuration
aws lambda update-function-configuration \
    --function-name rds-backup-stack-rds-backup \
    --handler bootstrap
```

### Deployment Script
Create a `deploy.sh` file:

```bash
#!/bin/bash
set -e

LAMBDA_FUNCTION="rds-backup-stack-rds-backup"
S3_BUCKET="rds-backup-stack-lambda"
S3_KEY="function.zip"

echo "Starting deployment process..."

# Upload to S3
echo "Uploading to S3..."
aws s3 cp function.zip "s3://${S3_BUCKET}/${S3_KEY}"

# Update Lambda function
echo "Updating Lambda function..."
aws lambda update-function-code \
    --function-name ${LAMBDA_FUNCTION} \
    --s3-bucket ${S3_BUCKET} \
    --s3-key ${S3_KEY}

# Update configuration
echo "Updating Lambda configuration..."
aws lambda update-function-configuration \
    --function-name ${LAMBDA_FUNCTION} \
    --handler bootstrap

echo "Deployment complete!"
```

## Setting up CloudWatch Events

### 1. Create Schedule Rule

```bash
# Create daily backup schedule (midnight UTC)
aws events put-rule \
    --name "RDSBackupSchedule" \
    --schedule-expression "cron(0 0 * * ? *)" \
    --state ENABLED

# For multiple schedules per day (e.g., every 12 hours)
aws events put-rule \
    --name "RDSBackupSchedule12h" \
    --schedule-expression "cron(0 */12 * * ? *)" \
    --state ENABLED
```

### 2. Add Lambda Target

```bash
# Replace <your-lambda-arn> with actual ARN
aws events put-targets \
    --rule "RDSBackupSchedule" \
    --targets "Id"="1","Arn"="<your-lambda-arn>"
```

### 3. Grant CloudWatch Events Permission

```bash
aws lambda add-permission \
    --function-name rds-backup-stack-rds-backup \
    --statement-id "CloudWatchEvents" \
    --action "lambda:InvokeFunction" \
    --principal "events.amazonaws.com" \
    --source-arn "<your-rule-arn>"
```

## Testing the Lambda Function

### Manual Invocation

```bash
# Invoke function and capture logs
aws lambda invoke \
    --function-name rds-backup-stack-rds-backup \
    --payload '{}' \
    --log-type Tail \
    --query 'LogResult' \
    --output text \
    output.txt | base64 --decode

# Check function output
cat output.txt
```

### Test Script
Create a `test.sh` file:

```bash
#!/bin/bash
set -e

LAMBDA_FUNCTION="rds-backup-stack-rds-backup"

echo "Testing Lambda function..."
aws lambda invoke \
    --function-name ${LAMBDA_FUNCTION} \
    --payload '{}' \
    --log-type Tail \
    --query 'LogResult' \
    --output text \
    test_output.txt | base64 --decode

echo "Test output:"
cat test_output.txt
```

## Environment Variables

Configure these environment variables in the Lambda console or using AWS CLI:

```bash
aws lambda update-function-configuration \
    --function-name rds-backup-stack-rds-backup \
    --environment "Variables={
        SOURCE_REGION=us-west-2,
        TARGET_REGION=us-east-1,
        DB_IDENTIFIER=my-database,
        SOURCE_BUCKET=source-backup-bucket,
        TARGET_BUCKET=target-backup-bucket,
        KMS_KEY_ID=mrk-1234567890abcdef,
        EXPORT_ROLE_ARN=arn:aws:iam::123456789012:role/rds-s3-export,
        ADMIN_EMAIL=admin@example.com,
        KEEP_SOURCE_SNAPSHOT=true,
        STORE_TO_SOURCE_S3=true
    }"
```

## Monitoring and Logging

### CloudWatch Log Groups

Monitor logs at:
```
/aws/lambda/rds-backup-stack-rds-backup
```

### CloudWatch Metrics

Set up CloudWatch Alarms for:
- Error rates
- Duration thresholds
- Throttles
- Concurrent executions

Example alarm creation:
```bash
aws cloudwatch put-metric-alarm \
    --alarm-name "RDSBackupErrors" \
    --alarm-description "Alert on backup errors" \
    --metric-name "Errors" \
    --namespace "AWS/Lambda" \
    --statistic "Sum" \
    --period 300 \
    --threshold 1 \
    --comparison-operator "GreaterThanThreshold" \
    --evaluation-periods 1 \
    --dimensions "Name=FunctionName,Value=rds-backup-stack-rds-backup" \
    --alarm-actions "<your-sns-topic-arn>"
```

## Troubleshooting

### Common Issues and Solutions

1. **Function Times Out**
   - Increase Lambda timeout in configuration
   - Check RDS snapshot size
   - Verify network connectivity

2. **Permission Errors**
   - Verify IAM role permissions
   - Check KMS key policies
   - Validate S3 bucket policies

3. **CloudWatch Events Not Triggering**
   - Verify rule schedule expression
   - Check rule state (ENABLED/DISABLED)
   - Validate target configuration

### Useful Commands

```bash
# Check Lambda logs
aws logs get-log-events \
    --log-group-name "/aws/lambda/rds-backup-stack-rds-backup" \
    --log-stream-name "$(aws logs describe-log-streams \
        --log-group-name "/aws/lambda/rds-backup-stack-rds-backup" \
        --order-by LastEventTime \
        --descending \
        --limit 1 \
        --query 'logStreams[0].logStreamName' \
        --output text)"

# Check function configuration
aws lambda get-function \
    --function-name rds-backup-stack-rds-backup

# Check CloudWatch Events rule
aws events describe-rule \
    --name "RDSBackupSchedule"
```

## Security Considerations

1. **KMS Key Rotation**
   - Enable automatic key rotation
   - Monitor key usage

2. **S3 Bucket Security**
   - Enable encryption
   - Configure lifecycle policies
   - Set up access logging

3. **Network Security**
   - Configure VPC endpoints if needed
   - Set up security groups
   - Monitor network access logs

## Best Practices

1. **Version Control**
   - Tag Lambda versions
   - Use Git for code management
   - Document changes

2. **Monitoring**
   - Set up comprehensive CloudWatch alarms
   - Monitor costs
   - Track backup success rates

3. **Maintenance**
   - Regular dependency updates
   - Security patches
   - Performance optimization

## Support and Resources

- AWS Lambda Documentation
- CloudWatch Events Documentation
- AWS CLI Reference
- Community Forums and Support

Remember to regularly update this guide as you make changes to the deployment process or add new features to the Lambda function.