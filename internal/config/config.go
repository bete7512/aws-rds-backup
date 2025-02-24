package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	SourceRegion       string
	TargetRegion       string
	DBIdentifier       string
	SourceBucket       string
	TargetBucket       string
	KMSKeyID           string
	ExportRoleARN      string
	KeepSourceSnapshot bool
	StoreToSourceS3    bool
	AdminEmail         string
}

func Load() *Config {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		// Try to load from the current directory and parent directories
		workDir, _ := os.Getwd()
		for i := 0; i < 3; i++ { // Try up to 3 levels up
			envPath := filepath.Join(workDir, ".env")
			err = godotenv.Load(envPath)
			if err == nil {
				break
			}
			workDir = filepath.Dir(workDir)
		}
		
		if err != nil {
			log.Println("Warning: .env file not found, using existing environment variables")
		}
	}

	requiredEnvVars := []string{
		"SOURCE_REGION",
		"TARGET_REGION",
		"DB_IDENTIFIER",
		"SOURCE_BUCKET",
		"TARGET_BUCKET",
		"KMS_KEY_ID",
		"EXPORT_ROLE_ARN",
		"ADMIN_EMAIL",
		"KEEP_SOURCE_SNAPSHOT",
		"STORE_TO_SOURCE_S3",
	}

	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			log.Fatalf("Missing required environment variable: %s", envVar)
		}
	}

	return &Config{
		SourceRegion:       os.Getenv("SOURCE_REGION"),
		TargetRegion:       os.Getenv("TARGET_REGION"),
		DBIdentifier:       os.Getenv("DB_IDENTIFIER"),
		SourceBucket:       os.Getenv("SOURCE_BUCKET"),
		TargetBucket:       os.Getenv("TARGET_BUCKET"),
		KMSKeyID:           os.Getenv("KMS_KEY_ID"),
		ExportRoleARN:      os.Getenv("EXPORT_ROLE_ARN"),
		AdminEmail:         os.Getenv("ADMIN_EMAIL"),
		KeepSourceSnapshot: os.Getenv("KEEP_SOURCE_SNAPSHOT") == "true",
		StoreToSourceS3:    os.Getenv("STORE_TO_SOURCE_S3") == "true",
	}
}
// package config

// import (
// 	"log"
// 	"os"
// )

// type Config struct {
// 	SourceRegion       string
// 	TargetRegion       string
// 	DBIdentifier       string
// 	SourceBucket       string
// 	TargetBucket       string
// 	KMSKeyID           string
// 	ExportRoleARN      string
// 	KeepSourceSnapshot bool
// 	StoreToSourceS3    bool
// 	AdminEmail         string
// }

// func Load() *Config {
// 	requiredEnvVars := []string{
// 		"SOURCE_REGION",
// 		"TARGET_REGION",
// 		"DB_IDENTIFIER",
// 		"SOURCE_BUCKET",
// 		"TARGET_BUCKET",
// 		"KMS_KEY_ID",
// 		"EXPORT_ROLE_ARN",
// 		"ADMIN_EMAIL",
// 		"KEEP_SOURCE_SNAPSHOT",
// 		"STORE_TO_SOURCE_S3",
// 	}

// 	for _, envVar := range requiredEnvVars {
// 		if os.Getenv(envVar) == "" {
// 			log.Fatalf("Missing required environment variable: %s", envVar)
// 		}
// 	}

// 	return &Config{
// 		SourceRegion:       os.Getenv("SOURCE_REGION"),
// 		TargetRegion:       os.Getenv("TARGET_REGION"),
// 		DBIdentifier:       os.Getenv("DB_IDENTIFIER"),
// 		SourceBucket:       os.Getenv("SOURCE_BUCKET"),
// 		TargetBucket:       os.Getenv("TARGET_BUCKET"),
// 		KMSKeyID:           os.Getenv("KMS_KEY_ID"),
// 		ExportRoleARN:      os.Getenv("EXPORT_ROLE_ARN"),
// 		AdminEmail:         os.Getenv("ADMIN_EMAIL"),
// 		KeepSourceSnapshot: os.Getenv("KEEP_SOURCE_SNAPSHOT") == "true",
// 		StoreToSourceS3:    os.Getenv("STORE_TO_SOURCE_S3") == "true",
// 	}
// }