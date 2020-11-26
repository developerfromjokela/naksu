package host

import (
	"fmt"
	"runtime"

	"naksu/log"
	"naksu/mebroutines"

	"github.com/intel-go/cpuid"
	"github.com/mackerelio/go-osstat/memory"

	humanize "github.com/dustin/go-humanize"
)

// host can be used to get information of the host machine

// LowDiskSizeError is an error returned by CheckFreeDisk()
type LowDiskSizeError struct {
	Err     string
	LowPath string
	LowSize uint64
}

func (e *LowDiskSizeError) Error() string {
	return fmt.Sprintf("path %s has low disk size: %d (%s)", e.LowPath, e.LowSize, humanize.Bytes(e.LowSize))
}

// IsHWVirtualisationCPU returns true if CPU supports hardware virtualisation
// This does not detect whether the support is turned in BIOS
// See IsHWVirtualisation()
func IsHWVirtualisationCPU() bool {
	if cpuid.HasFeature(cpuid.VMX) {
		log.Debug("Hardware virtualisation is supported by CPU (VT-x, CPU flag VMX)")
		return true
	}

	if cpuid.HasExtraFeature(cpuid.SVM) {
		log.Debug("Hardware virtualisation is supported by CPU (AMD-V, CPU flag SVM)")
		return true
	}

	log.Debug("Hardware virtualisation is not supported by CPU")
	return false
}

// GetCPUCoreCount returns number of CPU cores
func GetCPUCoreCount() int {
	return runtime.NumCPU()
}

// GetMemory returns system RAM (in megabytes)
func GetMemory() (uint64, error) {
	memory, err := memory.Get()
	if err != nil {
		return 0, err
	}

	return memory.Total / (1024 * 1024), nil
}

// CheckFreeDisk checks that all of the listed directories have more than
// given limit free disk space. If a directory has less than required disk space
// the returned error has prefix "low:" followed by a failed path. The uint64 returns free disk space
// of this location.
func CheckFreeDisk(limit uint64, directories []string) error {
	log.Debug(fmt.Sprintf("CheckFreeDisk: %v", directories))

	for _, thisDirectory := range directories {
		freeDisk, err := mebroutines.GetDiskFree(thisDirectory)

		if err != nil {
			log.Debug(fmt.Sprintf("CheckFreeDisk could not get free disk for path '%s': %v", thisDirectory, err))
		} else {
			log.Debug(fmt.Sprintf("CheckFreeDisk: %s (%d bytes, %s)", thisDirectory, freeDisk, humanize.Bytes(freeDisk)))

			if freeDisk < limit {
				return &LowDiskSizeError{"disk size is too low", thisDirectory, freeDisk}
			}
		}
	}

	return nil
}
