package factory

import (
	"context"
	"testing"
)

func TestWaitForWorkerLeaseStopsBeforeAcquire(t *testing.T) {
	stop := make(chan struct{})
	close(stop)
	manager := &Manager{instanceID: "test-instance"}
	if manager.waitForWorkerLease(context.Background(), "managed:123", 123, stop) {
		t.Fatal("expected closed stop channel to abort lease wait")
	}
}
