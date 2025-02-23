package backup

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	awsinternal "github.com/unplank/rds-backup-lambda/internal/aws"
	"github.com/unplank/rds-backup-lambda/internal/config"
)

func CreateAndExportSnapshotInSourceRegion(ctx context.Context, clients *awsinternal.AWSClients, config *config.Config, kmsKeyArn string, result *Result) (string, error) {
	snapshotID := fmt.Sprintf("backup-%s-%s", config.DBIdentifier, time.Now().Format("2006-01-02-15-04-05"))

	log.Printf("Creating snapshot: %s", snapshotID)
	_, err := clients.SourceRDS.CreateDBSnapshot(ctx, &rds.CreateDBSnapshotInput{
		DBInstanceIdentifier: aws.String(config.DBIdentifier),
		DBSnapshotIdentifier: aws.String(snapshotID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create snapshot: %w", err)
	}

	waiter := rds.NewDBSnapshotAvailableWaiter(clients.SourceRDS)
	if err := waiter.Wait(ctx, &rds.DescribeDBSnapshotsInput{
		DBSnapshotIdentifier: aws.String(snapshotID),
	}, 2*time.Hour); err != nil {
		return "", fmt.Errorf("error waiting for snapshot: %w", err)
	}

	log.Printf("Exporting snapshot to S3")
	if config.StoreToSourceS3 {
		exportTask, err := exportSnapshotToS3(ctx, clients.SourceS3, clients.SourceRDS, snapshotID, config.SourceBucket, kmsKeyArn, config.ExportRoleARN)
		if err != nil {
			return "", err
		}

		result.S3Location = fmt.Sprintf("s3://%s/%s", config.SourceBucket, exportTask)
	}

	return snapshotID, nil
}

func CopyAndExportSnapshotToTargetRegion(ctx context.Context, clients *awsinternal.AWSClients, config *config.Config, sourceSnapshotID string, targetKMSKeyArn string, result *Result) (string, error) {
	targetSnapshotID, err := copySnapshotToTargetRegion(ctx, clients.SourceRDS, clients.TargetRDS, sourceSnapshotID, targetKMSKeyArn)
	if err != nil {
		return "", err
	}

	exportTask, err := exportSnapshotToS3(ctx, clients.TargetS3, clients.TargetRDS, targetSnapshotID, config.TargetBucket, targetKMSKeyArn, config.ExportRoleARN)
	if err != nil {
		return "", err
	}

	result.S3Location += fmt.Sprintf("\ns3://%s/%s", config.TargetBucket, exportTask)
	return targetSnapshotID, nil
}

func exportSnapshotToS3(ctx context.Context, s3Client *s3.Client, rdsClient *rds.Client, snapshotID, bucket, kmsKeyArn, roleArn string) (string, error) {
	snapshot, err := rdsClient.DescribeDBSnapshots(ctx, &rds.DescribeDBSnapshotsInput{
		DBSnapshotIdentifier: aws.String(snapshotID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe snapshot: %w", err)
	}

	log.Printf("Starting export task for snapshot: %s", snapshotID)
	exportTask := fmt.Sprintf("export-%s", snapshotID)
	_, err = rdsClient.StartExportTask(ctx, &rds.StartExportTaskInput{
		ExportTaskIdentifier: aws.String(exportTask),
		IamRoleArn:           aws.String(roleArn),
		KmsKeyId:             aws.String(kmsKeyArn),
		S3BucketName:         aws.String(bucket),
		SourceArn:            snapshot.DBSnapshots[0].DBSnapshotArn,
	})
	if err != nil {
		return "", fmt.Errorf("failed to start export task: %w", err)
	}

	for {
		describeOutput, err := rdsClient.DescribeExportTasks(ctx, &rds.DescribeExportTasksInput{
			ExportTaskIdentifier: aws.String(exportTask),
		})
		if err != nil {
			return "", fmt.Errorf("error describing export tasks: %w", err)
		}
		if len(describeOutput.ExportTasks) == 0 {
			return "", fmt.Errorf("no export task found with identifier %s", exportTask)
		}

		status := *describeOutput.ExportTasks[0].Status
		log.Printf("Export task status: %s", status)

		if status == "COMPLETE" {
			break
		}
		if status == "FAILED" {
			failureMsg := "Unknown failure"
			if describeOutput.ExportTasks[0].FailureCause != nil {
				failureMsg = *describeOutput.ExportTasks[0].FailureCause
			}
			return "", fmt.Errorf("export task %s failed: %s", exportTask, failureMsg)
		}
		time.Sleep(5 * time.Minute)
	}

	return exportTask, nil
}

func copySnapshotToTargetRegion(ctx context.Context, sourceRDS *rds.Client, targetRDS *rds.Client, sourceSnapshotID string, targetKMSKeyArn string) (string, error) {
	snapshot, err := sourceRDS.DescribeDBSnapshots(ctx, &rds.DescribeDBSnapshotsInput{
		DBSnapshotIdentifier: aws.String(sourceSnapshotID),
	})
	if err != nil {
		return "", fmt.Errorf("failed to describe source snapshot: %w", err)
	}

	if len(snapshot.DBSnapshots) == 0 {
		return "", fmt.Errorf("no snapshot found with ID: %s", sourceSnapshotID)
	}

	sourceSnapshotArn := snapshot.DBSnapshots[0].DBSnapshotArn
	targetSnapshotID := fmt.Sprintf("copy-%s", sourceSnapshotID)

	log.Printf("Copying snapshot to target region: %s", targetSnapshotID)
	_, err = targetRDS.CopyDBSnapshot(ctx, &rds.CopyDBSnapshotInput{
		SourceDBSnapshotIdentifier: sourceSnapshotArn,
		TargetDBSnapshotIdentifier: aws.String(targetSnapshotID),
		KmsKeyId:                   aws.String(targetKMSKeyArn),
		CopyTags:                   aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("failed to start snapshot copy: %w", err)
	}

	waiter := rds.NewDBSnapshotAvailableWaiter(targetRDS)
	err = waiter.Wait(ctx, &rds.DescribeDBSnapshotsInput{
		DBSnapshotIdentifier: aws.String(targetSnapshotID),
	}, 2*time.Hour)
	if err != nil {
		return "", fmt.Errorf("error waiting for snapshot: %w", err)
	}

	return targetSnapshotID, nil
}

func DeleteSnapshot(ctx context.Context, rdsClient *rds.Client, snapshotID string) error {
	_, err := rdsClient.DeleteDBSnapshot(ctx, &rds.DeleteDBSnapshotInput{
		DBSnapshotIdentifier: aws.String(snapshotID),
	})
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}
	return nil
}

func CleanupOldSnapshots(ctx context.Context, clients *awsinternal.AWSClients, config *config.Config) error {
	cutoffTime := time.Now().AddDate(0, 0, -15)

	if err := deleteOldSnapshots(ctx, clients.SourceRDS, "backup-"+config.DBIdentifier, cutoffTime); err != nil {
		return fmt.Errorf("failed to cleanup source region snapshots: %w", err)
	}

	if err := deleteOldSnapshots(ctx, clients.TargetRDS, "copy-backup-"+config.DBIdentifier, cutoffTime); err != nil {
		return fmt.Errorf("failed to cleanup target region snapshots: %w", err)
	}

	return nil
}

func deleteOldSnapshots(ctx context.Context, rdsClient *rds.Client, prefix string, cutoffTime time.Time) error {
	input := &rds.DescribeDBSnapshotsInput{
		SnapshotType: aws.String("manual"),
	}

	paginator := rds.NewDescribeDBSnapshotsPaginator(rdsClient, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list snapshots: %w", err)
		}

		for _, snapshot := range output.DBSnapshots {
			if !strings.HasPrefix(*snapshot.DBSnapshotIdentifier, prefix) {
				continue
			}

			if snapshot.SnapshotCreateTime.Before(cutoffTime) {
				log.Printf("Deleting old snapshot: %s (created: %s)",
					*snapshot.DBSnapshotIdentifier,
					snapshot.SnapshotCreateTime.Format(time.RFC3339))

				_, err := rdsClient.DeleteDBSnapshot(ctx, &rds.DeleteDBSnapshotInput{
					DBSnapshotIdentifier: snapshot.DBSnapshotIdentifier,
				})
				if err != nil {
					log.Printf("Warning: Failed to delete snapshot %s: %v",
						*snapshot.DBSnapshotIdentifier, err)
				}
			}
		}
	}

	return nil
}
