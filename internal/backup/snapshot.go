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
    maxRetries := 12 // Will try for up to 1 hour (12 * 5 minutes)
    var lastErr error
    
    for i := 0; i < maxRetries; i++ {
        // Check instance state
        instance, err := clients.SourceRDS.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{
            DBInstanceIdentifier: aws.String(config.DBIdentifier),
        })
        if err != nil {
            lastErr = fmt.Errorf("failed to describe DB instance: %w", err)
            log.Printf("Error checking instance state: %v. Retry %d/%d", err, i+1, maxRetries)
            time.Sleep(5 * time.Minute)
            continue
        }

        if len(instance.DBInstances) == 0 {
            return "", fmt.Errorf("DB instance not found: %s", config.DBIdentifier)
        }

        status := *instance.DBInstances[0].DBInstanceStatus

        // If backing up, check for existing snapshot from today
        if status == "backing-up" {
            snapshots, err := clients.SourceRDS.DescribeDBSnapshots(ctx, &rds.DescribeDBSnapshotsInput{
                DBInstanceIdentifier: aws.String(config.DBIdentifier),
                SnapshotType:        aws.String("manual"),
            })
            if err != nil {
                lastErr = fmt.Errorf("failed to describe snapshots: %w", err)
                log.Printf("Error checking snapshots: %v. Retry %d/%d", err, i+1, maxRetries)
                time.Sleep(5 * time.Minute)
                continue
            }

            today := time.Now().Format("2006-01-02")
            for _, snap := range snapshots.DBSnapshots {
                if strings.HasPrefix(*snap.DBSnapshotIdentifier, "backup-"+config.DBIdentifier+"-"+today) {
                    log.Printf("Found existing snapshot from today: %s", *snap.DBSnapshotIdentifier)
                    
                    // Wait for the existing snapshot to be available
                    waiter := rds.NewDBSnapshotAvailableWaiter(clients.SourceRDS)
                    if err := waiter.Wait(ctx, &rds.DescribeDBSnapshotsInput{
                        DBSnapshotIdentifier: snap.DBSnapshotIdentifier,
                    }, 2*time.Hour); err != nil {
                        lastErr = fmt.Errorf("error waiting for existing snapshot: %w", err)
                        continue
                    }

                    // Export the existing snapshot if needed
                    if config.StoreToSourceS3 {
                        exportTask, err := exportSnapshotToS3(ctx, clients.SourceS3, clients.SourceRDS, 
                            *snap.DBSnapshotIdentifier, config.SourceBucket, kmsKeyArn, config.ExportRoleARN)
                        if err != nil {
                            return "", err
                        }
                        result.S3Location = fmt.Sprintf("s3://%s/%s", config.SourceBucket, exportTask)
                    }

                    return *snap.DBSnapshotIdentifier, nil
                }
            }
        }

        // Proceed only if instance is available
        if status == "available" {
            snapshotID := fmt.Sprintf("backup-%s-%s", config.DBIdentifier, time.Now().Format("2006-01-02-15-04-05"))
            log.Printf("Creating snapshot: %s", snapshotID)
            
            _, err = clients.SourceRDS.CreateDBSnapshot(ctx, &rds.CreateDBSnapshotInput{
                DBInstanceIdentifier: aws.String(config.DBIdentifier),
                DBSnapshotIdentifier: aws.String(snapshotID),
            })
            if err != nil {
                lastErr = fmt.Errorf("failed to create snapshot: %w", err)
                log.Printf("Error creating snapshot: %v. Retry %d/%d", err, i+1, maxRetries)
                time.Sleep(5 * time.Minute)
                continue
            }

            // Wait for the new snapshot to be available
            waiter := rds.NewDBSnapshotAvailableWaiter(clients.SourceRDS)
            if err := waiter.Wait(ctx, &rds.DescribeDBSnapshotsInput{
                DBSnapshotIdentifier: aws.String(snapshotID),
            }, 2*time.Hour); err != nil {
                lastErr = fmt.Errorf("error waiting for snapshot: %w", err)
                log.Printf("Error waiting for snapshot: %v. Retry %d/%d", err, i+1, maxRetries)
                time.Sleep(5 * time.Minute)
                continue
            }

            log.Printf("Exporting snapshot to S3")
            if config.StoreToSourceS3 {
                exportTask, err := exportSnapshotToS3(ctx, clients.SourceS3, clients.SourceRDS, 
                    snapshotID, config.SourceBucket, kmsKeyArn, config.ExportRoleARN)
                if err != nil {
                    return "", err
                }
                result.S3Location = fmt.Sprintf("s3://%s/%s", config.SourceBucket, exportTask)
            }

            return snapshotID, nil
        }

        log.Printf("DB instance %s is in %s state. Waiting 5 minutes before retry (%d/%d)",
            config.DBIdentifier, status, i+1, maxRetries)
        time.Sleep(5 * time.Minute)
    }

    if lastErr != nil {
        return "", fmt.Errorf("max retries reached with last error: %w", lastErr)
    }
    return "", fmt.Errorf("timed out waiting for DB instance to become available after %d retries", maxRetries)
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
    targetSnapshotID := fmt.Sprintf("copy-%s", sourceSnapshotID)

    // First check if the snapshot already exists
    existingSnapshot, err := targetRDS.DescribeDBSnapshots(ctx, &rds.DescribeDBSnapshotsInput{
        DBSnapshotIdentifier: aws.String(targetSnapshotID),
    })
    if err == nil && len(existingSnapshot.DBSnapshots) > 0 {
        // Snapshot exists, check its status
        status := *existingSnapshot.DBSnapshots[0].Status
        if status == "available" {
            log.Printf("Snapshot %s already exists and is available", targetSnapshotID)
            return targetSnapshotID, nil
        }
        // If snapshot exists but not available, wait for it
        waiter := rds.NewDBSnapshotAvailableWaiter(targetRDS)
        err = waiter.Wait(ctx, &rds.DescribeDBSnapshotsInput{
            DBSnapshotIdentifier: aws.String(targetSnapshotID),
        }, 2*time.Hour)
        if err == nil {
            return targetSnapshotID, nil
        }
        // If waiting failed, try to delete and recreate
        _ = DeleteSnapshot(ctx, targetRDS, targetSnapshotID)
    }

    // Original copy logic
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
// func copySnapshotToTargetRegion(ctx context.Context, sourceRDS *rds.Client, targetRDS *rds.Client, sourceSnapshotID string, targetKMSKeyArn string) (string, error) {
// 	snapshot, err := sourceRDS.DescribeDBSnapshots(ctx, &rds.DescribeDBSnapshotsInput{
// 		DBSnapshotIdentifier: aws.String(sourceSnapshotID),
// 	})
// 	if err != nil {
// 		return "", fmt.Errorf("failed to describe source snapshot: %w", err)
// 	}

// 	if len(snapshot.DBSnapshots) == 0 {
// 		return "", fmt.Errorf("no snapshot found with ID: %s", sourceSnapshotID)
// 	}

// 	sourceSnapshotArn := snapshot.DBSnapshots[0].DBSnapshotArn
// 	targetSnapshotID := fmt.Sprintf("copy-%s", sourceSnapshotID)

// 	log.Printf("Copying snapshot to target region: %s", targetSnapshotID)
// 	_, err = targetRDS.CopyDBSnapshot(ctx, &rds.CopyDBSnapshotInput{
// 		SourceDBSnapshotIdentifier: sourceSnapshotArn,
// 		TargetDBSnapshotIdentifier: aws.String(targetSnapshotID),
// 		KmsKeyId:                   aws.String(targetKMSKeyArn),
// 		CopyTags:                   aws.Bool(true),
// 	})
// 	if err != nil {
// 		return "", fmt.Errorf("failed to start snapshot copy: %w", err)
// 	}

// 	waiter := rds.NewDBSnapshotAvailableWaiter(targetRDS)
// 	err = waiter.Wait(ctx, &rds.DescribeDBSnapshotsInput{
// 		DBSnapshotIdentifier: aws.String(targetSnapshotID),
// 	}, 2*time.Hour)
// 	if err != nil {
// 		return "", fmt.Errorf("error waiting for snapshot: %w", err)
// 	}

// 	return targetSnapshotID, nil
// }

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
