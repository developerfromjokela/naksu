// Package mebroutines contains general routines used by various MEB helper utilities
package mebroutines

import (
	"errors"
	"fmt"
	"io"
	golog "log"
	"os"
	"path/filepath"
	"runtime"

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

// RemoveDir removes directory and all its contents. It returns an err in
// case of errors. See also RemoveDirAndLogErrors()
func RemoveDir(path string) error {
	err := os.RemoveAll(path)
	return err
}

// RemoveDirAndLogErrors tries to remove directory and all its contents
// Instead of returning errors on failed removals it logs files and directories
// which could not be removed. This is useful for avoid unnecessary error messages
// when deleting VirtualBox log files which are locked by the VirtualBox process.
// See also RemoveDir()
func RemoveDirAndLogErrors(topPath string) {
	paths := []string{}

	err := filepath.Walk(topPath,
		func(newPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			paths = append(paths, newPath)
			return nil
		})

	if err != nil {
		log.Debug("RemoveDirAndLogErrors could not remove directory %s: %v", topPath, err)
		return
	}

	for n := len(paths) - 1; n >= 0; n-- {
		err := os.Remove(paths[n])
		if err != nil {
			log.Debug("Could not remove %s: %v", paths[n], err)
		}
	}
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

// GetVirtualBoxHiddenDirectory returns path to global VirtualBox settings
// https://docs.oracle.com/en/virtualization/virtualbox/6.0/admin/vboxconfigdata.html
func GetVirtualBoxHiddenDirectory() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(GetHomeDirectory(), "Library", "VirtualBox")
	case "linux":
		return filepath.Join(GetHomeDirectory(), ".config", "VirtualBox")
	case "windows":
		return filepath.Join(GetHomeDirectory(), ".VirtualBox")
	default:
		log.Debug("GetVirtualBoxHiddenDirectory() could not detect execution environment")
	}

	return filepath.Join(GetHomeDirectory(), ".VirtualBox")
}

// GetVirtualBoxVMsDirectory returns "VirtualBox VMs" path from under home directory
func GetVirtualBoxVMsDirectory() string {
	return filepath.Join(GetHomeDirectory(), "VirtualBox VMs")
}

// GetZipImagePath returns path of an unzipped VM image
func GetZipImagePath() string {
	return filepath.Join(GetKtpDirectory(), "naksu_last_image.zip")
}

func GetVDIImagePath() string {
	return filepath.Join(GetKtpDirectory(), "naksu_ktp_disk.vdi")
}

// GetImagePath returns a path of a raw VM image
func GetImagePath() string {
	return filepath.Join(GetKtpDirectory(), "naksu_last_image.dd")
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
	log.Error(message)

	// Show libui box if main window has been set with Set_main_window
	if mainWindow != nil {
		ui.QueueMain(func() {
			ui.MsgBoxError(mainWindow, xlate.Get("Error"), message)
		})
	}
}

// ShowTranslatedErrorMessage translates given error message and shows it with ShowErrorMessage()
func ShowTranslatedErrorMessage(str string, vars ...interface{}) {
	ShowErrorMessage(xlate.Get(str, vars...))
}

// ShowTranslatedErrorMessageAndPassError can be used to show a general error popup
// and return the given error upstream:
// return mebroutines.ShowTranslatedErrorMessageAndPassError("General error: %v", errors.New("Shit happened"))
func ShowTranslatedErrorMessageAndPassError(str string, err error) error {
	ShowTranslatedErrorMessage(str, err)
	return err
}

// ShowWarningMessage shows warning message popup to user
func ShowWarningMessage(message string) {
	log.Warning(message)

	// Show libui box if main window has been set with Set_main_window
	if mainWindow != nil {
		ui.QueueMain(func() {
			ui.MsgBox(mainWindow, xlate.Get("Warning"), message)
		})
	}
}

// ShowTranslatedWarningMessage translates given warning message and shows it with ShowWarningMessage()
func ShowTranslatedWarningMessage(str string, vars ...interface{}) {
	ShowWarningMessage(xlate.Get(str, vars...))
}

// ShowInfoMessage shows warning message popup to user
func ShowInfoMessage(message string) {
	log.Info(message)

	// Show libui box if main window has been set with Set_main_window
	if mainWindow != nil {
		ui.QueueMain(func() {
			ui.MsgBox(mainWindow, xlate.Get("Info"), message)
		})
	}
}

// ShowTranslatedInfoMessage translates given info message and shows it with ShowInfoMessage()
func ShowTranslatedInfoMessage(str string, vars ...interface{}) {
	ShowInfoMessage(xlate.Get(str, vars...))
}
