package main

import "testing"

func TestProcessRequiresDispatchStatus(t *testing.T) {
	err := process(WorkOrder{ID: "WO-42"}, &InfraiClient{})
	if err == nil || err.Error() != "id and dispatch_status are required" {
		t.Fatalf("expected validation error, got %v", err)
	}
}
