package config

import (
	"fmt"
	"naksu/mebroutines"
	"strconv"

	"github.com/go-ini/ini"
)

var cfg *ini.File

func setIfMissing(section string, key string, defaultValue string) {
	if !cfg.Section(section).HasKey(key) {
		cfg.Section(section).Key(key).SetValue(defaultValue)
	}
}

type defaultValue struct {
	section, key, value string
}

var defaults = []defaultValue{
	defaultValue{"common", "iniVersion", strconv.FormatInt(1, 10)},
	defaultValue{"common", "language", "fi"},
	defaultValue{"selfupdate", "disabled", strconv.FormatBool(false)},
}

func fillDefaults() {
	for _, defaultValue := range defaults {
		setIfMissing(defaultValue.section, defaultValue.key, defaultValue.value)
	}
}

func getDefault(section string, key string) string {
	for _, defaultValue := range defaults {
		if defaultValue.section == section && defaultValue.key == key {
			return defaultValue.value
		}
	}
	panic(fmt.Sprintf("Default for %v / %v is not defined!", section, key))
}

func getIniKey(section string, key string) *ini.Key {
	return cfg.Section(section).Key(key)
}

func getBoolean(section string, key string) bool {
	value, err := getIniKey(section, key).Bool()
	if err != nil {
		mebroutines.LogDebug(fmt.Sprintf("Parsing key %s / %s as bool failed", section, key))
		defaultValue := getDefault(section, key)
		value, err = strconv.ParseBool(defaultValue)
		if err != nil {
			panic(fmt.Sprintf("Default boolean parsing for %v / %v (%v) failed to parse to boolean!", section, key, defaultValue))
		}
		setValue(section, key, defaultValue)
	}
	return value
}

func getString(section string, key string) string {
	return getIniKey(section, key).String()
}

func setValue(section string, key string, value string) {
	cfg.Section(section).Key(key).SetValue(value)
	save()
}

// Load or initialize configuration to empty object
func Load() {
	var err error
	cfg, err = ini.Load("naksu.ini")
	if err != nil {
		mebroutines.LogDebug("naksu.ini not found, setting up empty config with defaults")
		cfg = ini.Empty()
	}
	fillDefaults()
	save()
}

func validateStringChoice(section string, key string, choices *map[string]bool) string {
	value := getString(section, key)
	_, ok := languages[value]
	if ok {
		return value
	}
	defaultValue := getDefault(section, key)
	mebroutines.LogDebug(fmt.Sprintf("Correcting malformed ini-key %v / %v to default value %v", section, key, defaultValue))
	setValue(section, key, defaultValue)
	return defaultValue
}

// Save configuration to disk
func save() {
	err := cfg.SaveTo("naksu.ini")
	if err != nil {
		mebroutines.LogDebug(fmt.Sprintf("naksu.ini save failed: %v", err))
	}
}

var languages = map[string]bool{
	"en": true,
	"fi": true,
	"sv": true,
}

// GetLanguage returns user language preference. defaults to fi
func GetLanguage() string {
	return validateStringChoice("common", "language", &languages)
}

// SetLanguage stores user language preference
func SetLanguage(language string) {
	_, ok := languages[language]
	if ok {
		setValue("common", "language", language)
	} else {
		setValue("common", "language", getDefault("common", "language"))
	}
}

// IsSelfUpdateDisabled returns true, if self-update functionality should be disabled
func IsSelfUpdateDisabled() bool {
	return getBoolean("selfupdate", "disabled")
}

// SetSelfUpdateDisabled sets the state of self-update functionality
func SetSelfUpdateDisabled(isSelfUpdateDisabled bool) {
	setValue("selfupdate", "disabled", strconv.FormatBool(isSelfUpdateDisabled))
}