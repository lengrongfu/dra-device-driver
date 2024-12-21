package main

import "fmt"

type VGpuInfo struct {
	UUID                  string `json:"uuid"`
	index                 int
	minor                 int
	migEnabled            bool
	memoryBytes           uint64
	productName           string
	brand                 string
	architecture          string
	cudaComputeCapability string
	driverVersion         string
	cudaDriverVersion     string
}

func ConvGpuInfoToVGpuInfo(info *GpuInfo) *VGpuInfo {
	return &VGpuInfo{
		UUID:                  info.UUID,
		index:                 info.index,
		minor:                 info.minor,
		migEnabled:            info.migEnabled,
		memoryBytes:           info.memoryBytes,
		productName:           info.productName,
		brand:                 info.brand,
		architecture:          info.architecture,
		cudaComputeCapability: info.cudaComputeCapability,
		driverVersion:         info.driverVersion,
		cudaDriverVersion:     info.cudaDriverVersion,
	}
}

func (v *VGpuInfo) CanonicalName() string {
	return fmt.Sprintf("gpu-%d-mig-%d-%d-%d", d.parent.index, d.giInfo.ProfileId, d.placement.Start, d.placement.Size)
}
