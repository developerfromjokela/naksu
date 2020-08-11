package vboxmanage

import (
	"fmt"
	"time"
  "regexp"

	"github.com/paulusrobin/go-memory-cache/memory-cache"

	"naksu/log"
	"naksu/mebroutines"
  "naksu/constants"
)

var vBoxResponseCache memory_cache.Cache
var vBoxManageStarted int64

type VBoxCommand = []string

func CallRunVBoxManage(args VBoxCommand) (string, error) {
	// There is an ongoing VBoxManage call (break free after 240 loops)
	// This locking avoids executing multiple instances of VBoxManage at the same time. Calling
	// VBoxManage simulaneously tends to cause E_ACCESSDENIED errors from VBoxManage.
	tryCounter := 0
	for (vBoxManageStarted != 0) && (tryCounter < 240) {
		time.Sleep(500 * time.Millisecond)
		tryCounter++
		log.Debug(fmt.Sprintf("CallRunVBoxManage is waiting VBoxManage to exit (race condition lock count %d)", tryCounter))
	}

	vBoxManageStarted = time.Now().Unix()
	vBoxManageOutput, err := mebroutines.RunVBoxManage(args)
	vBoxManageStarted = 0

	return vBoxManageOutput, err
}

func MultipleCallRunVBoxManage(commands []VBoxCommand) error {
	for curCommand := 0; curCommand < len(commands); curCommand++ {
		_, err := CallRunVBoxManage(commands[curCommand])
		if err != nil {
			return err
		}
	}

	return nil
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

// GetVMInfoRegexp returns result of the given vmRegexp from the current VBoxManage showvminfo
// output. This function gets the output either from the cache or calls getVBoxManageOutput()
func GetVMInfoRegexp(boxName string, vmRegexp string) string {
	var rawVMInfo string

	ensureVBoxResponseCacheInitialised()

	rawVMInfoInterface, err := vBoxResponseCache.Get("showvminfo")
	if err != nil {
		rawVMInfo, err = CallRunVBoxManage([]string{"showvminfo", "--machinereadable", boxName})
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

	// Extract server name
	pattern := regexp.MustCompile(vmRegexp)
	result := pattern.FindStringSubmatch(rawVMInfo)

	if len(result) > 1 {
		return result[1]
	}

	return ""
}

func GetBoxProperty(boxName string, property string) string {
	ensureVBoxResponseCacheInitialised()

	propertyValue := ""

	propertyValueInterface, errCache := vBoxResponseCache.Get(property)
	if errCache != nil {
		output, errVBoxManage := CallRunVBoxManage([]string{"guestproperty", "get", boxName, property})
		if errVBoxManage != nil {
			log.Debug(fmt.Sprintf("Could not get VM guest property: %v", errVBoxManage))
			output = ""
		}

		propRegexp := regexp.MustCompile(`Value: (.+)`)
		propMatches := propRegexp.FindStringSubmatch(output)
		if len(propMatches) == 2 {
			propertyValue = propMatches[1]
		}

		errCacheSet := vBoxResponseCache.Set(property, propertyValue, constants.VBoxManageCacheTimeout)
		if errCacheSet == nil {
			log.Debug(fmt.Sprintf("Stored VM guest property %s to cache: %s", property, propertyValue))
		} else {
			log.Debug(fmt.Sprintf("Could not store VM guest property %s, value %s to cache: %v", property, propertyValue, errCacheSet))
		}
	} else {
		propertyValue = fmt.Sprintf("%v", propertyValueInterface)
		log.Debug(fmt.Sprintf("Got VM guest property %s from cache: %s", property, propertyValue))
	}

	return propertyValue
}
