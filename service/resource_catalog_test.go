package service

import (
	"context"
	"testing"
	"time"

	gcsclient "github.com/ReyRen/gcs-distill/internal/client/gcs"
	"github.com/ReyRen/gcs-distill/internal/config"
	"github.com/ReyRen/gcs-distill/internal/types"
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
	executor := config.ExecutorConfig{DataWorkers: []string{"worker"}}
	enrichResourceNodes(nodes, inventory, executor, now)
	if !nodes[0].Devices[0].Selectable || nodes[0].Devices[1].Selectable || nodes[0].Devices[1].State != "in_use" || nodes[0].Devices[2].State != "reserved" || !nodes[0].DataSupported {
		t.Fatalf("incorrect card state: %+v", nodes[0])
	}
	inventory.Nodes[0].ObservedAt = resourcePointer(now.Add(-3 * time.Minute))
	enrichResourceNodes(nodes, inventory, executor, now)
	if nodes[0].Devices[0].Selectable || nodes[0].Devices[0].State != "telemetry_stale" {
		t.Fatal("stale card marked selectable")
	}
	enrichResourceNodes(nodes, nil, executor, now)
	if nodes[0].Devices[0].State != "telemetry_unknown" || nodes[0].Devices[0].MemoryTotalMiB != nil {
		t.Fatal("missing measurement fabricated")
	}
}

func TestAscendCatalogPreservesNoncontiguousPhysicalIDs(t *testing.T) {
	now := time.Now()
	nodes := []AvailableNode{{Name: "arm", Available: true, WorkersXPUName: "910B", WorkersXPUCount: 2, EnableXPUIndices: []int{5}, NodeState: "ready", NodeAvailability: "active", NodeArchitecture: "aarch64"}}
	idle := gcsclient.DeviceResource{Index: 5, Name: "Ascend 910B", HardwareEnabled: resourcePointer(true), SoftwareEnabled: resourcePointer(true), MemoryTotalMiB: resourcePointer(uint64(32768)), ProcessCount: resourcePointer(0)}
	busy := idle
	busy.Index, busy.ProcessCount = 2, resourcePointer(1)
	inventory := &gcsclient.ResourceInventory{Nodes: []gcsclient.NodeResources{{Name: "arm", ObservedAt: &now, Devices: []gcsclient.DeviceResource{idle, busy}}}}
	executor := config.ExecutorConfig{Training: config.TrainingRuntime{Accelerator: config.AcceleratorAscend, Workers: []string{"arm"}}}
	enrichResourceNodes(nodes, inventory, executor, now)
	cards := nodes[0].Devices
	if len(cards) != 2 || cards[0].Index != 2 || cards[0].Selectable || cards[1].Index != 5 || !cards[1].Selectable {
		t.Fatalf("unexpected Ascend card inventory: %+v", cards)
	}
}

func TestARMDeploymentWithNoTrainingWorkersOnlyAcceptsDataJobs(t *testing.T) {
	resources := NewResourceService(nil, config.ExecutorConfig{
		Training: config.TrainingRuntime{Accelerator: config.AcceleratorAscend},
	})
	if err := resources.Validate(context.Background(), &types.ResourceRequest{}, false, true); err != nil {
		t.Fatal(err)
	}
	request := &types.ResourceRequest{SelectedResources: []types.SelectedResource{{NodeName: "arm", XPUIndices: []int{2}}}}
	if err := resources.Validate(context.Background(), request, true, true); err == nil {
		t.Fatal("training was accepted without an enabled worker")
	}
}
