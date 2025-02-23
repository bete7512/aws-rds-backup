package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/unplank/rds-backup-lambda/internal/backup"
	"github.com/unplank/rds-backup-lambda/internal/config"
	"github.com/unplank/rds-backup-lambda/internal/notification"
)

// type LambdaEvent struct {
// }

// func handleRequest(ctx context.Context, event LambdaEvent) error {
// 	cfg := config.Load()

// 	result := &backup.Result{
// 		DBIdentifier: cfg.DBIdentifier,
// 		BackupTime:   time.Now().Format(time.RFC3339),
// 	}

// 	err := backup.Perform(ctx, cfg, result)
// 	if err != nil {
// 		result.ErrorMessage = err.Error()
// 		if sendErr := notification.SendFailureEmail(cfg, result); sendErr != nil {
// 			log.Printf("Failed to send failure email: %v", sendErr)

// 		}
// 		return err
// 	}

// 	if sendErr := notification.SendSuccessEmail(cfg, result); sendErr != nil {
// 		log.Printf("Failed to send success email: %v", sendErr)
// 		return sendErr
// 	}
// 	log.Println("Backup completed successfully")

// 	return nil
// }

func main() {
	// lambda.Start(handleRequest)
	c := cron.New(cron.WithLocation(time.UTC),
		cron.WithChain(
			cron.Recover(cron.DefaultLogger),            // Recover from panics
			cron.SkipIfStillRunning(cron.DefaultLogger), // Skip if previous backup is still running
		))

	cfg := config.Load()
	// Schedule backup at midnight UTC
	_, err := c.AddFunc("0 0 * * *", func() {
		ctx := context.Background()
		result := &backup.Result{
			DBIdentifier: cfg.DBIdentifier,
			BackupTime:   time.Now().Format(time.RFC3339),
		}

		log.Printf("Starting database backup for %s", cfg.DBIdentifier)
		err := backup.Perform(ctx, cfg, result)
		if err != nil {
			result.ErrorMessage = err.Error()
			if sendErr := notification.SendFailureEmail(cfg, result); sendErr != nil {
				log.Printf("Failed to send failure email: %v", sendErr)
			}
			log.Printf("Backup failed: %v", err)
		} else {
			if sendErr := notification.SendSuccessEmail(cfg, result); sendErr != nil {
				log.Printf("Failed to send success email: %v", sendErr)
			}
			log.Printf("Backup completed successfully for %s", cfg.DBIdentifier)
		}
	})

	if err != nil {
		log.Fatalf("Error scheduling backup: %v", err)
	}

	// Start the scheduler
	c.Start()

	// Run backup immediately on startup
	ctx := context.Background()
	result := &backup.Result{
		DBIdentifier: cfg.DBIdentifier,
		BackupTime:   time.Now().Format(time.RFC3339),
	}

	log.Printf("Running initial backup for %s", cfg.DBIdentifier)
	err = backup.Perform(ctx, cfg, result)
	if err != nil {
		result.ErrorMessage = err.Error()
		if sendErr := notification.SendFailureEmail(cfg, result); sendErr != nil {
			log.Printf("Failed to send failure email: %v", sendErr)
		}
		log.Printf("Initial backup failed: %v", err)
	} else {
		if sendErr := notification.SendSuccessEmail(cfg, result); sendErr != nil {
			log.Printf("Failed to send success email: %v", sendErr)
		}
		log.Printf("Initial backup completed successfully for %s", cfg.DBIdentifier)
	}

	// Handle graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down backup scheduler...")
	ctx = c.Stop()
	<-ctx.Done()
	log.Println("Backup scheduler stopped successfully")
}
