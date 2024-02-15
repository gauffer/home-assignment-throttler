package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/gauffer/home-assignment-throttler/internal/service"
)

const maxRetries = 5

// BatchProcessError helps the client to keep track of processed items.
type BatchProcessError struct {
	Processed int
	Err       error
}

func (e *BatchProcessError) Error() string {
	return fmt.Sprintf("processed %d items: %v", e.Processed, e.Err)
}

func (e *BatchProcessError) Unwrap() error {
	return e.Err
}

// Client is the struct that implements the logic of the communication with the service.
// The client is capable of processing items in batches and allows mupltiple attempts to do that. 
// See: const maxRetries.
type Client struct {
	service service.Service
}

func NewClient(service service.Service) *Client {
	return &Client{service: service}
}

// ProcessBatchesWithRetry wraps call to the ProcessBatchesInternal.
// It will check limits of the serice and attempt to process data in batches.
// In case of an expected error it will check limits again, 
// and then try again.
// See: const maxRetries
func (c *Client) ProcessBatchesWithRetry(
	ctx context.Context,
	items []service.Item,
) error {
	retryStrategy := func(attempt int) time.Duration {
		// We may use exponential backoff instead.
		return time.Second * time.Duration(
			attempt,
		)
	}
	n, p := c.service.GetLimits()

	var processedTotal int
	attempt := 0
	for {
		err := c.ProcessBatchesInternal(ctx, items[processedTotal:], n, p)
		if err == nil {
			slog.Info(
				"Successful",
				"size",
				len(items),
				"current_shifted_size",
				len(items[processedTotal:]),
				"limit_size",
				n,
			)
			return nil
		}

		var bpe *BatchProcessError
		if errors.As(err, &bpe) {
			processedTotal += bpe.Processed
			slog.Warn(
				"Keeping track in error",
				"processed_successfully_before_error", bpe.Processed,
				"error", bpe.Err,
			)

			n, p = c.service.GetLimits()
		}

		attempt++
		if attempt > maxRetries {
			return fmt.Errorf("after %d attempts, last error: %w", attempt, err)
		}

		time.Sleep(retryStrategy(attempt))
	}
}

// ProcessBatchesInternal will atomically process each batch,
// but will error if the service is blocked for whatever reason.
// It will keep track of processed items count in a custom error,
// so that we would have an opportunity to implement retries.
func (c *Client) ProcessBatchesInternal(
	ctx context.Context,
	items []service.Item,
	n uint64,
	p time.Duration,
) error {
	batchSize := int(n)
	processedCount := 0

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := service.Batch(items[i:end])

		batchCtx, cancel := context.WithTimeout(ctx, p)
		err := c.service.Process(batchCtx, batch)
		// Tricky, we are sending cancellation to all
		// child contexts in the tree, could be dangerous.
		cancel()

		if err != nil {
			if errors.Is(err, service.ErrBlocked) {
				return &BatchProcessError{Processed: processedCount, Err: err}
			}
			// Well, we must have more than one err.
			slog.Error("Enexpected error", "err", err)
			os.Exit(1)
		}

		processedCount += len(batch)

		if i+batchSize < len(items) {
			time.Sleep(p)
		}
	}

	return nil
}
