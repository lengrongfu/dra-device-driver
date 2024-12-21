package v1alpha1

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Priority int

const (
	PriorityLow  Priority = 0
	PriorityHigh Priority = 999
)

const (
	DefaultCore             = 10
	DefaultMemory           = 1000
	DefaultMemoryPercentage = 100
	DefaultPriority         = PriorityLow
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VGpuConfig holds the set of parameters for configuring a GPU.
type VGpuConfig struct {
	metav1.TypeMeta  `json:",inline"`
	Core             int64    `json:"core"`
	Memory           int64    `json:"memory"`
	MemoryPercentage int64    `json:"memoryPercentage"`
	Priority         Priority `json:"priority"`
}

type VGpuScalingConfig struct {
	CoreScaling   float64
	MemoryScaling float64
}

// DefaultVGpuConfig provides the default VGPU configuration.
func DefaultVGpuConfig() *VGpuConfig {
	return &VGpuConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: GroupName + "/" + Version,
			Kind:       VGpuConfigKind,
		},
		Core:             DefaultCore,
		Memory:           DefaultMemory,
		MemoryPercentage: DefaultMemoryPercentage,
		Priority:         DefaultPriority,
	}
}

// Normalize updates a VGpuConfig config with implied default values based on other settings.
func (c *VGpuConfig) Normalize() error {
	if c.Core <= 0 {
		c.Core = DefaultCore
	}
	if c.Memory <= 0 {
		c.Memory = DefaultMemory
	}
	if c.MemoryPercentage < 0 || c.MemoryPercentage > 100 {
		c.MemoryPercentage = DefaultMemoryPercentage
	}
	if c.Priority < PriorityLow || c.Priority > PriorityHigh {
		c.Priority = DefaultPriority
	}
	return nil
}

// Validate ensures that GpuConfig has a valid set of values.
func (c *VGpuConfig) Validate() error {
	if c.Core <= 0 {
		return fmt.Errorf("core must be greater than 0")
	}
	if c.Memory <= 0 {
		return fmt.Errorf("memory must be greater than 0")
	}
	if c.MemoryPercentage < 0 || c.MemoryPercentage > 100 {
		return fmt.Errorf("memory percentage must be between 0 and 100")
	}
	if c.Priority < PriorityLow || c.Priority > PriorityHigh {
		return fmt.Errorf("priority must be between %d and %d", PriorityLow, PriorityHigh)
	}
	return nil
}
