package service

import (
	"context"
	"errors"
	"strings"
)

const (
	batchImageHoldRequestPrefix    = "batch_image_hold:"
	batchImageCaptureRequestPrefix = "batch_image_capture:"
	batchImageReleaseRequestPrefix = "batch_image_release:"
)

func BatchImageHoldRequestID(batchID string) string {
	return batchImageHoldRequestPrefix + strings.TrimSpace(batchID)
}

func BatchImageCaptureRequestID(batchID string) string {
	return batchImageCaptureRequestPrefix + strings.TrimSpace(batchID)
}

func BatchImageReleaseRequestID(batchID string) string {
	return batchImageReleaseRequestPrefix + strings.TrimSpace(batchID)
}

func reserveBatchImageSubscription(_ context.Context, _ UsageBillingRepository, job *BatchImageJob, _ string) error {
	if job == nil || job.SubscriptionID == nil || *job.SubscriptionID <= 0 {
		return ErrSubscriptionInvalid
	}
	return nil
}

func captureBatchImageSubscription(ctx context.Context, repo UsageBillingRepository, job *BatchImageJob, actualAmount float64, payloadHash string) error {
	if repo == nil {
		return ErrBatchImageSettlementBillingFailed.WithCause(errors.New("batch image billing repository is not configured"))
	}
	if job == nil || job.APIKeyID == nil || job.AccountID == nil || job.SubscriptionID == nil {
		return ErrSubscriptionInvalid
	}
	cmd := &UsageBillingCommand{
		RequestID:          BatchImageCaptureRequestID(job.BatchID),
		APIKeyID:           *job.APIKeyID,
		UserID:             job.UserID,
		AccountID:          *job.AccountID,
		SubscriptionID:     job.SubscriptionID,
		Model:              job.Model,
		BillingType:        BillingTypeSubscription,
		ImageCount:         job.SuccessCount,
		SubscriptionCost:   actualAmount,
		RequestPayloadHash: strings.TrimSpace(payloadHash),
	}
	cmd.Normalize()
	if _, err := repo.Apply(ctx, cmd); err != nil {
		return ErrBatchImageSettlementBillingFailed.WithCause(err)
	}
	return nil
}

func releaseBatchImageSubscription(context.Context, UsageBillingRepository, *BatchImageJob, string) error {
	return nil
}
