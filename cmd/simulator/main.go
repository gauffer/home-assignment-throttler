package main

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/gauffer/home-assignment-throttler/internal/client"
	"github.com/gauffer/home-assignment-throttler/internal/service"
)

type MockService struct {
	n           int
	p           time.Duration
	blockChance int
}

func (m *MockService) GetLimits() (n uint64, p time.Duration) {
	return uint64(m.n), p
}

func (m *MockService) Process(ctx context.Context, batch service.Batch) error {
	if m.blockChance == 0 {
		return nil
	}

	src := rand.NewSource(time.Now().UnixNano())
	rnd := rand.New(src) // #nosec G404

	if rnd.Intn(m.blockChance) < 1 {
		return service.ErrBlocked
	}
	return nil
}

type TestCase struct {
	Name             string
	BlockPercentage  int
	LimitN           int
	LimitP           time.Duration
	BatchSize        int
	WorkloadDuration time.Duration
	RatePerSecond    int
}

func main() {
	testCases := []TestCase{
		{
			Name:             "Walk",
			BlockPercentage:  0,
			LimitN:           10,
			LimitP:           1 * time.Second,
			BatchSize:        5,
			WorkloadDuration: 5 * time.Second,
			RatePerSecond:    1,
		},
		{
			Name:             "Sprint",
			BlockPercentage:  10,
			LimitN:           100,
			LimitP:           1 * time.Second,
			BatchSize:        20,
			WorkloadDuration: 5 * time.Second,
			RatePerSecond:    10,
		},
		{
			Name:             "Marathon",
			BlockPercentage:  10,
			LimitN:           1000,
			LimitP:           1 * time.Second,
			BatchSize:        200,
			WorkloadDuration: 10 * time.Second,
			RatePerSecond:    20,
		},
	}

	for _, tc := range testCases {
		runTestCase(tc)
	}
}

func runTestCase(tc TestCase) {
	service := &MockService{tc.LimitN, tc.LimitP, tc.BlockPercentage}
	client := client.NewClient(service)

	slog.Info(
		"starting",
		"test_case",
		tc.Name,
		"batch_size",
		tc.BatchSize,
		"duration",
		tc.WorkloadDuration,
		"rate_per_second",
		tc.RatePerSecond,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		tc.WorkloadDuration,
	)
	defer cancel()

	var wg sync.WaitGroup

	ticker := time.NewTicker(time.Second / time.Duration(tc.RatePerSecond))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			slog.Info(tc.Name + " simulation complete.")
			return
		case <-ticker.C:
			wg.Add(1)
			go simulateBatchProcessing(client, tc.BatchSize, &wg)
		}
	}
}

func simulateBatchProcessing(
	c *client.Client,
	batchSize int,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	items := make([]service.Item, batchSize)
	for i := range items {
		items[i] = service.Item{}
	}

	err := c.ProcessBatchesWithRetry(context.Background(), items)
	if err != nil {
		slog.Info("Error processing batch: %v\n", err)
	}
}
