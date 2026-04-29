package billing

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestManagerConcurrentLastUnitReservation(t *testing.T) {
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	repo := newFixtureRepo(t, now)
	manager := NewManagerWithClock(repo, func() time.Time { return now })
	ctx := context.Background()
	allowed := 0
	denied := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := manager.Reserve(ctx, ReserveInput{TenantID: fixtureTenantA, Category: CategoryRunLaunches, Amount: 1, OperationKey: RunOperationKey(fixtureTenantA, "", "run_last_"+string(rune('a'+i))), Hosted: true})
			mu.Lock()
			defer mu.Unlock()
			if err == nil && result.Allowed {
				allowed++
			} else if errors.Is(err, ErrQuotaDenied) {
				denied++
			}
		}(i)
	}
	wg.Wait()
	if allowed != 1 || denied != 3 {
		t.Fatalf("allowed=%d denied=%d, want 1/3", allowed, denied)
	}
}
