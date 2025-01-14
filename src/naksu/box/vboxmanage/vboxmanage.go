package vboxmanage

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	semver "github.com/blang/semver/v4"
	memory_cache "github.com/paulusrobin/go-memory-cache/memory-cache"

	"naksu/constants"
	"naksu/log"
	"naksu/mebroutines"
)

const vBoxManageOutputNoVMInstalled string = "Could not find a registered machine named"

// vBoxResponseCache is initialised by init() -> ensureVBoxResponseCacheInitialised()
var vBoxResponseCache memory_cache.Cache
var vBoxManageStarted int64

type VBoxCommand = []string

// nolint:gochecknoinits
func init() {
	ensureVBoxResponseCacheInitialised()
}

func RunCommand(args VBoxCommand) (string, error) {
	return runCommand(args, true)
}

func RunCommandWithoutLogging(args VBoxCommand) (string, error) {
	return runCommand(args, false)
}

func runCommand(args VBoxCommand, logOutput bool) (string, error) {
	// There is an ongoing VBoxManage call (break free after 240 loops)
	// This locking avoids executing multiple instances of VBoxManage at the same time. Calling
	// VBoxManage simulaneously tends to cause E_ACCESSDENIED errors from VBoxManage.
	tryCounter := 0
	for (vBoxManageStarted != 0) && (tryCounter < 240) {
		time.Sleep(500 * time.Millisecond)
		tryCounter++
		log.Debug(fmt.Sprintf("RunCommand is waiting VBoxManage to exit (race condition lock count %d)", tryCounter))
	}

	vBoxManageStarted = time.Now().Unix()
	vBoxManageOutput, err := runVBoxManage(args, logOutput)
	vBoxManageStarted = 0

	return vBoxManageOutput, err
}

func RunCommands(commands []VBoxCommand) error {
	for curCommand := 0; curCommand < len(commands); curCommand++ {
		_, err := RunCommand(commands[curCommand])
		if err != nil {
			return err
		}
	}

	return nil
}

// runVBoxManage runs vboxmanage command with given arguments
func runVBoxManage(args []string, logOutput bool) (string, error) {
	vboxmanagepathArr := []string{getVBoxManagePath()}
	runArgs := append(vboxmanagepathArr, args...)
	vBoxManageOutput, err := mebroutines.RunAndGetOutput(runArgs, logOutput)
	if err != nil {
		command := strings.Join(runArgs, " ")
		logError := func(output string, err error) {
			log.Debug(fmt.Sprintf("Failed to execute %s (%v), complete output:", command, err))
			log.Debug(output)
		}

		logError(vBoxManageOutput, err)

		fixed, fixErr := detectAndFixDuplicateHardDiskProblem(vBoxManageOutput)
		if !fixed && fixErr != nil {
			log.Debug(fmt.Sprintf("Failed to fix duplicate hard disk problem with command %s: (%v)", command, fixErr))
			return "", fmt.Errorf("failed to execute %s: %v", command, err)
		}

		// We need to re-run the command only if problem was fixed
		if fixed {
			log.Debug(fmt.Sprintf("Retrying '%s' after fixing problem", command))
			vBoxManageOutput, err = mebroutines.RunAndGetOutput(runArgs, logOutput)
			if err != nil {
				logError(vBoxManageOutput, err)
			}
		}
	}

	return vBoxManageOutput, err
}

func ensureVBoxResponseCacheInitialised() {
	var err error

	if vBoxResponseCache == nil {
		vBoxResponseCache, err = memory_cache.New()
		if err != nil {
			log.Debug(fmt.Sprintf("Fatal error: Failed to initialise memory cache: %v", err))
			panic(err)
		}
	}
}

func ResetVBoxResponseCache() {
	vBoxResponseCache = nil
	ensureVBoxResponseCacheInitialised()
}

// GetVMInfoByRegexp returns result of the given vmRegexp from the current VBoxManage showvminfo
// output. This function gets the output either from the cache or calls getVBoxManageOutput()
func GetVMInfoByRegexp(vmName string, vmRegexp string) string {
	rawVMInfo := getVMInfo(vmName)

	// Extract server name
	pattern := regexp.MustCompile(vmRegexp)
	result := pattern.FindStringSubmatch(rawVMInfo)

	if len(result) > 1 {
		return result[1]
	}

	return ""
}

// Get "showvminfo" output from vBoxResponseCache (if present) or VBoxManage
func getVMInfo(vmName string) string {
	var rawVMInfo string

	rawVMInfoInterface, err := vBoxResponseCache.Get("showvminfo")
	if err != nil {
		rawVMInfo, err = RunCommandWithoutLogging([]string{"showvminfo", "--machinereadable", vmName})
		if err != nil {
			log.Debug(fmt.Sprintf("Could not get VM info: %v", err))
			rawVMInfo = ""
		}

		errCache := vBoxResponseCache.Set("showvminfo", rawVMInfo, constants.VBoxManageCacheTimeout)
		if errCache != nil {
			log.Debug(fmt.Sprintf("Could not store VM info to cache: %v", errCache))
		}
	} else {
		rawVMInfo = fmt.Sprintf("%v", rawVMInfoInterface)
	}

	return rawVMInfo
}

func getVBoxManageVersionSemanticPart() (string, error) {
	output, errVBM := RunCommand([]string{"--version"})
	if errVBM != nil {
		log.Debug(fmt.Sprintf("GetVBoxManageVersion() failed to get VBoxManage version: %v", errVBM))
		return "", fmt.Errorf("failed to get vboxmanage version: %v", errVBM)
	}

	re := regexp.MustCompile(`^(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(output)
	if matches != nil {
		return matches[1], nil
	}

	return "", fmt.Errorf("could not find semantic version string from vboxmanage version '%s'", output)
}

func GetVBoxManageVersion() (semver.Version, error) {
	cachedVBoxManageVersion, errCache := vBoxResponseCache.Get("vboxmanageversion")
	if errCache != nil {
		vBoxManageVersionString, errVersionString := getVBoxManageVersionSemanticPart()
		if errVersionString != nil {
			log.Debug(fmt.Sprintf("GetVBoxManageVersion() could not get VBoxManage version: %v", errVersionString))
			return semver.Version{}, errVersionString
		}

		vBoxManageVersion, errSemVer := semver.Make(vBoxManageVersionString)
		if errSemVer != nil {
			log.Debug(fmt.Sprintf("GetVBoxManageVersion() got VBoxManage version code '%s' but it is not semantic version number: %v", vBoxManageVersionString, errSemVer))
			return semver.Version{}, fmt.Errorf("vboxmanage version %s is not a semantic version number: %v", vBoxManageVersionString, errSemVer)
		}

		errCache = vBoxResponseCache.Set("vboxmanageversion", vBoxManageVersion.String(), constants.VBoxManageCacheTimeout)
		if errCache != nil {
			log.Debug(fmt.Sprintf("GetVBoxManageVersion() could not store version to cache: %v", errCache))
		}

		return vBoxManageVersion, nil
	}

	cachedVBoxManageVersionSemVer, err := semver.Make(fmt.Sprintf("%v", cachedVBoxManageVersion))

	if err != nil {
		return semver.Version{}, fmt.Errorf("getting semantic version object from '%v' caused error: %v", cachedVBoxManageVersion, err)
	}

	return cachedVBoxManageVersionSemVer, nil
}

func GetVMProperty(vmName string, property string) string {
	propertyValue := ""

	propertyValueInterface, errCache := vBoxResponseCache.Get(property)
	if errCache != nil {
		output, errVBoxManage := RunCommand([]string{"guestproperty", "get", vmName, property})
		if errVBoxManage != nil {
			log.Debug(fmt.Sprintf("Could not get VM guest property '%s': %v", property, errVBoxManage))
			output = ""
		}

		propRegexp := regexp.MustCompile(`Value:\s*(\w+)`)
		propMatches := propRegexp.FindStringSubmatch(output)
		if propMatches != nil {
			propertyValue = propMatches[1]
		}

		errCacheSet := vBoxResponseCache.Set(property, propertyValue, constants.VBoxManageCacheTimeout)
		if errCacheSet == nil {
			log.Debug(fmt.Sprintf("Stored VM guest property '%s' value '%s' to cache", property, propertyValue))
		} else {
			log.Debug(fmt.Sprintf("Could not store VM guest property '%s', value '%s' to cache: %v", property, propertyValue, errCacheSet))
		}
	} else {
		propertyValue = fmt.Sprintf("%v", propertyValueInterface)
		log.Debug(fmt.Sprintf("Got VM guest property %s from cache: %s", property, propertyValue))
	}

	return propertyValue
}

func getVMStateFromOutput(output string) string {
	re := regexp.MustCompile(`VMState="(.+)"`)
	result := re.FindStringSubmatch(output)

	if len(result) > 1 {
		return result[1]
	}

	return ""
}

func getVMState(vmName string) (string, error) {
	vmState, err := vBoxResponseCache.Get("vmstate")
	if err != nil {
		rawVMInfo, err := RunCommandWithoutLogging([]string{"showvminfo", "--machinereadable", vmName})

		// Check whether VM is installed
		if strings.Contains(rawVMInfo, vBoxManageOutputNoVMInstalled) {
			log.Debug("When trying to get VM state, VM is not installed")
			return "", nil
		}

		// Process other VBoxManage errors
		if err != nil {
			log.Debug(fmt.Sprintf("When trying to get VM state, could not get VM info: %v", err))
			return "", err
		}

		// Extract state string
		vmState = getVMStateFromOutput(rawVMInfo)
		if vmState == "" {
			log.Debug("Could not find VM state from the VM info")
			return "", errors.New("could not find vm state from the vm info")
		}

		errCache := vBoxResponseCache.Set("vmstate", vmState, constants.VBoxRunningCacheTimeout)
		if errCache != nil {
			log.Debug(fmt.Sprintf("Could not store VM state to cache: %v", errCache))
		}
	}

	return fmt.Sprintf("%v", vmState), nil
}

// IsVMRunning returns true if given VM is currently running
func IsVMRunning(vmName string) (bool, error) {
	vmState, err := getVMState(vmName)

	if vmState != "<nil>" {
		// Log only messages with content to avoid log spam
		log.Debug(fmt.Sprintf("vboxmanage.IsVMRunning() got following state string: '%s'", vmState))
	}

	if err != nil {
		return false, err
	}

	return vmState == "running", nil
}

// IsVMInstalled returns true if given VM has been installed
func IsVMInstalled(vmName string) (bool, error) {
	rawVMInfo, err := RunCommandWithoutLogging([]string{"showvminfo", "--machinereadable", vmName})

	if err != nil {
		if strings.Contains(rawVMInfo, vBoxManageOutputNoVMInstalled) {
			log.Debug("vboxmanage.IsVMInstalled: Box is not installed")
			return false, nil
		}

		// Other error, return it to the caller
		return false, err
	}

	// We got the showvminfo all right, so the machine is installed
	return true, nil
}

// IsIstalled returns true if VBoxManage has been installed
func IsInstalled() bool {
	var vboxmanagepath = getVBoxManagePath()

	if vboxmanagepath == "" {
		log.Debug("Could not get VBoxManage path")
		return false
	}

	vBoxManageVersion, err := RunCommand([]string{"--version"})
	if err != nil {
		// No VBoxManage was found
		log.Debug("VBoxManage was not found")
		return false
	}

	log.Debug(fmt.Sprintf("VBoxManage version: %s", vBoxManageVersion))
	return true
}
