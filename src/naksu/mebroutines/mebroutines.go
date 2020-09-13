// Package mebroutines contains general routines used by various MEB helper utilities
package mebroutines

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	golog "log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"naksu/log"
	"naksu/xlate"

	"github.com/andlabs/ui"
	"github.com/mitchellh/go-homedir"
)

var mainWindow *ui.Window

// Close gracefully handles closing of closable item. defer Close(item)
func Close(c io.Closer) {
	err := c.Close()
	if err != nil {
		golog.Fatal(err)
	}
}

// RunAndGetOutput runs command with arguments and returns output as a string
func RunAndGetOutput(commandArgs []string) (string, error) {
	log.Debug(fmt.Sprintf("RunAndGetOutput: %s", strings.Join(commandArgs, " ")))
	/* #nosec */
	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)

	out, err := cmd.CombinedOutput()

	if err != nil {
		log.Debug(fmt.Sprintf(xlate.Get("command failed: %s (%v)"), strings.Join(commandArgs, " "), err))
	}

	if out != nil {
		log.Debug("RunAndGetOutput returns combined STDOUT and STDERR:")
		log.Debug(string(out))
	} else {
		log.Debug("RunAndGetOutput returned NIL as combined STDOUT and STDERR")
	}

	return string(out), err
}

func getFileMode(path string) (os.FileMode, error) {
	fi, err := os.Lstat(path)
	if err == nil {
		return fi.Mode(), nil
	}

	return 0, err
}

// ExistsDir returns true if given path exists
func ExistsDir(path string) bool {
	mode, err := getFileMode(path)

	if err == nil && mode.IsDir() {
		return true
	}

	return false
}

// ExistsFile returns true if given file exists
func ExistsFile(path string) bool {
	mode, err := getFileMode(path)

	if err == nil && mode.IsRegular() {
		return true
	}

	return false
}

// ExistsCharDevice returns true if given file is a Linux device file
func ExistsCharDevice(path string) bool {
	mode, err := getFileMode(path)

	return err == nil && mode&os.ModeDevice != 0 && mode&os.ModeCharDevice != 0
}

// CreateDir creates new directory
func CreateDir(path string) error {
	var err = os.Mkdir(path, os.ModePerm)
	return err
}

// CreateFile creates empty new file
func CreateFile(path string) error {
	f, err := os.Create(path)
	if err == nil {
		defer Close(f)
	}
	return err
}

// RemoveDir removes directory and all its contents
func RemoveDir(path string) error {
	err := os.RemoveAll(path)
	return err
}

// CopyFile copies existing file
func CopyFile(src, dst string) (err error) {
	log.Debug(fmt.Sprintf("Copying file %s to %s", src, dst))

	if !ExistsFile(src) {
		log.Debug("Copying failed, could not find source file")
		return errors.New("could not find source file")
	}

	/* #nosec */
	in, err := os.Open(src)
	if err != nil {
		log.Debug(fmt.Sprintf("Copying failed while opening source file: %v", err))
		return fmt.Errorf("could not open source file: %v", err)
	}
	defer Close(in)

	out, err := os.Create(dst)
	if err != nil {
		log.Debug(fmt.Sprintf("Copying failed while opening destination file: %v", err))
		return fmt.Errorf("could not open destination file: %v", err)
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return fmt.Errorf("error when copying data: %v", err)
	}
	err = out.Sync()
	if err != nil {
		log.Debug(fmt.Sprintf("Copying failed while syncing destination file: %v", err))
		return fmt.Errorf("error when syncing destination file: %v", err)
	}

	return nil
}

// GetHomeDirectory returns home directory path
func GetHomeDirectory() string {
	homeDir, err := homedir.Dir()

	if err != nil {
		panic("Could not get home directory")
	}

	return homeDir
}

// GetKtpDirectory returns ktp-directory path from under home directory
func GetKtpDirectory() string {
	return filepath.Join(GetHomeDirectory(), "ktp")
}

// GetMebshareDirectory returns ktp-jako path from under home directory
func GetMebshareDirectory() string {
	return filepath.Join(GetHomeDirectory(), "ktp-jako")
}

// GetVirtualBoxHiddenDirectory returns ".VirtualBox" path from under home directory
func GetVirtualBoxHiddenDirectory() string {
	return filepath.Join(GetHomeDirectory(), ".VirtualBox")
}

// GetVirtualBoxVMsDirectory returns "VirtualBox VMs" path from under home directory
func GetVirtualBoxVMsDirectory() string {
	return filepath.Join(GetHomeDirectory(), "VirtualBox VMs")
}

// GetTempFilename creates a temporary file, closes it and returns its filename
func GetTempFilename() (string, error) {
	tempFile, err := ioutil.TempFile(os.TempDir(), "naksu-")
	if err != nil {
		log.Debug(fmt.Sprintf("Failed to create temporary file: %v", err))
		return "", err
	}

	defer tempFile.Close()

	return tempFile.Name(), nil
}

// chdir changes current working directory to the given directory
func chdir(chdirTo string) bool {
	log.Debug(fmt.Sprintf("chdir %s", chdirTo))
	err := os.Chdir(chdirTo)
	if err != nil {
		log.Debug(fmt.Sprintf("Could not chdir to %s: %v", chdirTo, err))
		return false
	}

	return true
}

// ChdirHomeDirectory changes current working directory to home directory
func ChdirHomeDirectory() bool {
	return chdir(GetHomeDirectory())
}

// SetMainWindow sets libui main window pointer used by ShowErrorMessage and ShowWarningMessage
func SetMainWindow(win *ui.Window) {
	mainWindow = win
}

// ShowErrorMessage shows error message popup to user
func ShowErrorMessage(message string) {
	fmt.Printf("ERROR: %s\n\n", message)
	log.Debug(fmt.Sprintf("ERROR: %s", message))

	// Show libui box if main window has been set with Set_main_window
	if mainWindow != nil {
		ui.QueueMain(func() {
			ui.MsgBoxError(mainWindow, xlate.Get("Error"), message)
		})
	}
}

// ShowWarningMessage shows warning message popup to user
func ShowWarningMessage(message string) {
	fmt.Printf("WARNING: %s\n", message)
	log.Debug(fmt.Sprintf("WARNING: %s", message))

	// Show libui box if main window has been set with Set_main_window
	if mainWindow != nil {
		ui.QueueMain(func() {
			ui.MsgBox(mainWindow, xlate.Get("Warning"), message)
		})
	}
}

// ShowInfoMessage shows warning message popup to user
func ShowInfoMessage(message string) {
	fmt.Printf("INFO: %s\n", message)
	log.Debug(fmt.Sprintf("INFO: %s", message))

	// Show libui box if main window has been set with Set_main_window
	if mainWindow != nil {
		ui.QueueMain(func() {
			ui.MsgBox(mainWindow, xlate.Get("Info"), message)
		})
	}
}
