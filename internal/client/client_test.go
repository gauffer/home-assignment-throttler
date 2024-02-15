package client

import (
	"context"

	"testing"
	"time"

	"github.com/gauffer/home-assignment-throttler/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockService is a mock of the Service interface.
// Generally I do anything to avoid mocks, but here they are suitable solution.
type MockService struct {
	mock.Mock
}

func (m *MockService) GetLimits() (n uint64, p time.Duration) {
	args := m.Called()
	return args.Get(0).(uint64), args.Get(1).(time.Duration)
}

func (m *MockService) Process(ctx context.Context, batch service.Batch) error {
	args := m.Called(ctx, batch)
	return args.Error(0)
}

func TestProcessBatchesWithRetryHappyPath(t *testing.T) {
	mockService := new(MockService)
	mockService.On("GetLimits").Return(uint64(10), 1*time.Second)
	mockService.On("Process", mock.Anything, mock.Anything).Return(nil)

	client := NewClient(mockService)
	items := make([]service.Item, 20)

	err := client.ProcessBatchesWithRetry(context.Background(), items)
	assert.NoError(t, err)

	mockService.AssertExpectations(t)
}

func TestProcessBatchesWithRetryFlexible(t *testing.T) {
	items := []service.Item{{}, {}, {}} // Three items to process
	ctx := context.Background()

	mockService := new(MockService)
	mockService.On("GetLimits").Return(uint64(1), 10*time.Millisecond)

	// The first call is blocked, and subsequent calls succeed
	mockService.On("Process", mock.Anything, mock.AnythingOfType("Batch")).
		Return(service.ErrBlocked).
		Once()
	mockService.On("Process", mock.Anything, mock.AnythingOfType("Batch")).
		Return(nil)

	client := NewClient(mockService)

	err := client.ProcessBatchesWithRetry(ctx, items)

	// Check if the function eventually succeeds or handles retries correctly
	assert.NoError(
		t,
		err,
		"The retry logic should eventually succeed after handling errors appropriately",
	)

	// Verify that the mock's expectations are met
	mockService.AssertExpectations(t)
}
