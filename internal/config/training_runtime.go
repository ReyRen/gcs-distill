package config

import (
	"slices"
	"strings"
)

const (
	AcceleratorNVIDIA  = "nvidia"
	AcceleratorAscend  = "ascend"
	NVIDIADataImage    = "gcs/distill-data:2.0.0-aeb8ca3-x86_64"
	NVIDIATrainerImage = "gcs/distill-trainer:0.9.3-cu124-vl-x86_64"
)

// TrainingRuntime describes the workers on which the configured trainer image
// has been installed. The ARM and x86 images target different accelerators.
type TrainingRuntime struct {
	Accelerator string
	Workers     []string
}

func (r TrainingRuntime) DeviceLabel() string {
	if r.Accelerator == AcceleratorAscend {
		return "NPU"
	}
	return "GPU"
}

func (r TrainingRuntime) Architecture() string {
	if r.Accelerator == AcceleratorAscend {
		return "arm64"
	}
	return "amd64"
}

func (r TrainingRuntime) AllowsWorker(name string) bool {
	return (len(r.Workers) == 0 && r.Accelerator != AcceleratorAscend) || slices.Contains(r.Workers, name)
}

func (r TrainingRuntime) Supports(arch, device string) bool {
	arch, device = strings.ToLower(arch), strings.ToLower(device)
	if r.Accelerator == AcceleratorAscend {
		return (arch == "arm64" || arch == "aarch64") && strings.Contains(device, "910")
	}
	return (arch == "amd64" || arch == "x86_64") && strings.Contains(device, "nvidia")
}
