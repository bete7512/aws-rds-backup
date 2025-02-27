package backup

import (
	"context"
	"fmt"
	"log"

	"github.com/unplank/rds-backup-lambda/internal/aws"
	"github.com/unplank/rds-backup-lambda/internal/config"
)

type Result struct {
	DBIdentifier     string
	SnapshotID       string
	BackupTime       string
	S3Location       string
	SourceS3Location *string
	ErrorMessage     string
}

func Perform(ctx context.Context, cfg *config.Config, result *Result) error {
	clients, err := aws.NewClients(ctx, cfg.SourceRegion, cfg.TargetRegion)
	if err != nil {
		return err
	}

	sourceKMSKeyArn, err := aws.GetKMSKeyARN(ctx, cfg.SourceRegion, cfg.KMSKeyID)
	if err != nil {
		return fmt.Errorf("failed to get source KMS key ARN: %w", err)
	}
	targetKMSKeyArn, err := aws.GetKMSKeyARN(ctx, cfg.TargetRegion, cfg.KMSKeyID)
	if err != nil {
		return fmt.Errorf("failed to get target KMS key ARN: %w", err)
	}

	sourceSnapshotID, err := CreateAndExportSnapshotInSourceRegion(ctx, clients, cfg, sourceKMSKeyArn, result)
	if err != nil {
		return err
	}
	result.SnapshotID = sourceSnapshotID

	targetSnapshotID, err := CopyAndExportSnapshotToTargetRegion(ctx, clients, cfg, sourceSnapshotID, targetKMSKeyArn, result)
	if err != nil {
		return err
	}

	if err := CleanupOldSnapshots(ctx, clients, cfg); err != nil {
		log.Printf("Failed to cleanup old snapshots: %v", err)
	}

	if !cfg.KeepSourceSnapshot {
		if err := DeleteSnapshot(ctx, clients.SourceRDS, sourceSnapshotID); err != nil {
			log.Printf("Warning: Failed to delete source snapshot: %v", err)
		}
	}

	log.Println("Backup completed successfully")
	log.Printf("Snapshot ID: %s", targetSnapshotID)
	log.Printf("S3 Location: %s", result.S3Location)

	return nil
}
