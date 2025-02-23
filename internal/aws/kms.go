package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func GetKMSKeyARN(ctx context.Context, region, keyID string) (string, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", fmt.Errorf("unable to load SDK config: %v", err)
	}

	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("unable to get caller identity: %v", err)
	}

	if strings.HasPrefix(keyID, "mrk-") {
		return fmt.Sprintf("arn:aws:kms:%s:%s:key/%s", region, *identity.Account, keyID), nil
	}

	if strings.HasPrefix(keyID, "arn:aws:kms") {
		return keyID, nil
	}

	return fmt.Sprintf("arn:aws:kms:%s:%s:key/%s", region, *identity.Account, keyID), nil
}

func VerifyKMSKey(ctx context.Context, cfg aws.Config, keyArn string) error {
	kmsClient := kms.NewFromConfig(cfg)

	resp, err := kmsClient.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: aws.String(keyArn),
	})
	if err != nil {
		return fmt.Errorf("failed to describe KMS key: %w", err)
	}

	if !resp.KeyMetadata.Enabled {
		return fmt.Errorf("KMS key is disabled")
	}

	return nil
}