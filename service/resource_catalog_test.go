package service

import (
	"testing"
	"time"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
)

func resourcePointer[T any](v T) *T { return &v }

func TestResourceCardsNeverTreatEnabledAsPhysicallyIdle(t *testing.T) {
	now := time.Now()
	nodes := []AvailableNode{{Name: "worker", Available: true, WorkersXPUName: "NVIDIA RTX 3090", WorkersXPUCount: 3, EnableXPUIndices: []int{0, 1}, NodeState: "ready", NodeAvailability: "active", NodeArchitecture: "amd64"}}
	device := gcsclient.DeviceResource{Name: "NVIDIA RTX 3090", HardwareEnabled: resourcePointer(true), SoftwareEnabled: resourcePointer(true), MemoryTotalMiB: resourcePointer(uint64(24576)), ProcessCount: resourcePointer(0)}
	cards := []gcsclient.DeviceResource{device, device, device}
	for i := range cards {
		cards[i].Index = i
	}
	cards[1].ProcessCount = resourcePointer(2)
	inventory := &gcsclient.ResourceInventory{Nodes: []gcsclient.NodeResources{{Name: "worker", ObservedAt: &now, Devices: cards}}}
	enrichResourceNodes(nodes, inventory, []string{"worker"}, now)
	if !nodes[0].Devices[0].Selectable || nodes[0].Devices[1].Selectable || nodes[0].Devices[1].State != "in_use" || nodes[0].Devices[2].State != "reserved" || !nodes[0].DataSupported {
		t.Fatalf("incorrect card state: %+v", nodes[0])
	}
	inventory.Nodes[0].ObservedAt = resourcePointer(now.Add(-3 * time.Minute))
	enrichResourceNodes(nodes, inventory, nil, now)
	if nodes[0].Devices[0].Selectable || nodes[0].Devices[0].State != "telemetry_stale" {
		t.Fatal("stale card marked selectable")
	}
	enrichResourceNodes(nodes, nil, nil, now)
	if nodes[0].Devices[0].State != "telemetry_unknown" || nodes[0].Devices[0].MemoryTotalMiB != nil {
		t.Fatal("missing measurement fabricated")
	}
}
