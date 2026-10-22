package gcs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func TestGPUSelectionUsesPhysicalProcessesAndSchedulerReservations(t *testing.T) {
	now := time.Now().UTC()
	card := DeviceResource{Index: 6, Name: "NVIDIA RTX 3090", HardwareEnabled: ptr(true), SoftwareEnabled: ptr(true), MemoryTotalMiB: ptr(uint64(24576)), ProcessCount: ptr(0)}
	node := NodeResources{Name: "worker", Architecture: "amd64", ObservedAt: &now, Devices: []DeviceResource{card}}
	enabled := []int{6}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/brain":
			_ = json.NewEncoder(w).Encode(map[string]any{"brain_workerenablearray": []string{"worker"}, "brain_workers": []any{map[string]any{"brain_workers_name": "worker", "brain_workers_address": "10.0.0.1", "brain_workers_enable_xpuarray": enabled}}})
		case "/resources":
			_ = json.NewEncoder(w).Encode(ResourceInventory{Nodes: []NodeResources{node}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, time.Second)
	selection := []SelectedResource{{NodeName: "worker", XPUIndices: []int{6}}}
	if err := client.ValidateGPUSelection(context.Background(), selection, true); err != nil {
		t.Fatal(err)
	}
	node.Devices[0].ProcessCount = ptr(1)
	if err := client.ValidateGPUSelection(context.Background(), selection, true); err == nil {
		t.Fatal("external compute process treated as idle")
	}
	if err := client.ValidateGPUSelection(context.Background(), selection, false); err != nil {
		t.Fatalf("pending snapshots should not reserve cards: %v", err)
	}
	node.Devices[0].ProcessCount = ptr(0)
	enabled = nil
	if err := client.ValidateGPUSelection(context.Background(), selection, true); err == nil {
		t.Fatal("scheduler reservation ignored")
	}
	enabled = []int{6}
	node.ObservedAt = ptr(now.Add(-3 * time.Minute))
	if err := client.ValidateGPUSelection(context.Background(), selection, true); err == nil {
		t.Fatal("stale measurement accepted")
	}
	node.ObservedAt = &now
	node.Devices[0].ProcessCount = nil
	if err := client.ValidateGPUSelection(context.Background(), selection, true); err == nil {
		t.Fatal("unknown process count treated as zero")
	}
	node.Architecture = "arm64"
	if err := client.ValidateGPUSelection(context.Background(), selection, false); err == nil {
		t.Fatal("unsupported training architecture accepted")
	}
}
