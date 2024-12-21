package main

import (
	"fmt"
	nvdev "github.com/NVIDIA/go-nvlib/pkg/nvlib/device"
)

func (l deviceLib) enumerateVGpuDevices(config *Config) (AllocatableDevices, error) {
	if err := l.Init(); err != nil {
		return nil, err
	}
	defer l.alwaysShutdown()

	devices := make(AllocatableDevices)
	deviceClasses := config.flags.deviceClasses
	err := l.VisitDevices(func(i int, d nvdev.Device) error {
		gpuInfo, err := l.getGpuInfo(i, d)
		if err != nil {
			return fmt.Errorf("error getting info for GPU %d: %w", i, err)
		}

		if deviceClasses.Has(VGpuDeviceType) && !gpuInfo.migEnabled {
			deviceInfo := &AllocatableDevice{
				VGPU: ConvGpuInfoToVGpuInfo(gpuInfo),
			}
			devices[gpuInfo.CanonicalName()] = deviceInfo
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("error visiting devices: %w", err)
	}

	return devices, nil
}
