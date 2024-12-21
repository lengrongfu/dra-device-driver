package v1alpha1

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultCore   = 10
	DefaultMemory = 1000
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VGpuConfig holds the set of parameters for configuring a GPU.
type VGpuConfig struct {
	metav1.TypeMeta  `json:",inline"`
	Core             int64  `json:"core"`
	Memory           int64  `json:"memory"`
	MemoryPercentage int64  `json:"memoryPercentage"`
	Priority         string `json:"priority"`
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
		SplitCount: DefaultSplitCount,
		Scaling: VGpuScalingConfig{
			CoreScaling:   DefaultCoreScaling,
			MemoryScaling: DefaultMemoryScaling,
		},
	}
}

// Normalize updates a VGpuConfig config with implied default values based on other settings.
func (c *VGpuConfig) Normalize() error {
	if c.SplitCount == 0 {
		c.SplitCount = DefaultSplitCount
	}
	if c.Scaling.CoreScaling == 0 {
		c.Scaling.CoreScaling = DefaultCoreScaling
	}
	if c.Scaling.MemoryScaling == 0 {
		c.Scaling.MemoryScaling = DefaultMemoryScaling
	}
	return nil
}

// Validate ensures that GpuConfig has a valid set of values.
func (c *VGpuConfig) Validate() error {
	return c.Scaling.Validate()
}

func (sc *VGpuScalingConfig) Validate() error {
	if sc.CoreScaling <= 0 || sc.MemoryScaling <= 0 {
		return fmt.Errorf("invalide VGPU Scaling config")
	}
	return nil
}
