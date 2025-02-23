package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type AWSClients struct {
	SourceRDS *rds.Client
	TargetRDS *rds.Client
	SourceS3  *s3.Client
	TargetS3  *s3.Client
}

func NewClients(ctx context.Context, sourceRegion, targetRegion string) (*AWSClients, error) {
	sourceCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(sourceRegion))
	if err != nil {
		return nil, fmt.Errorf("failed to load source region config: %w", err)
	}

	targetCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(targetRegion))
	if err != nil {
		return nil, fmt.Errorf("failed to load target region config: %w", err)
	}

	return &AWSClients{
		SourceRDS: rds.NewFromConfig(sourceCfg),
		TargetRDS: rds.NewFromConfig(targetCfg),
		SourceS3:  s3.NewFromConfig(sourceCfg),
		TargetS3:  s3.NewFromConfig(targetCfg),
	}, nil
}
