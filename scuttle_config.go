package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultAdminAPIPort = 15000
const defaultQuitAPIPort = 15020

// ScuttleConfig ... represents Scuttle's configuration based on environment variables or defaults.
type ScuttleConfig struct {
	LoggingEnabled          bool
	EnvoyAdminAPI           string
	StartWithoutEnvoy       bool
	WaitForEnvoyTimeout     time.Duration
	IstioQuitAPI            string
	NeverKillIstio          bool
	NeverKillIstioOnFailure bool
	GenericQuitEndpoints    []string
	QuitRequestTimeout      time.Duration
	QuitWithoutEnvoyTimeout time.Duration
}

func log(message string) {
	if config.LoggingEnabled {
		fmt.Printf("%s scuttle: %s\n", time.Now().UTC().Format("2006-01-02T15:04:05Z"), message)
	}
}

func getConfig() ScuttleConfig {
	loggingEnabled := getBoolFromEnv("SCUTTLE_LOGGING", true, false)
	config := ScuttleConfig{
		// Logging enabled by default, disabled if "false"
		LoggingEnabled:          loggingEnabled,
		EnvoyAdminAPI:           getStringFromEnv("ENVOY_ADMIN_API", "", loggingEnabled),
		StartWithoutEnvoy:       getBoolFromEnv("START_WITHOUT_ENVOY", false, loggingEnabled),
		WaitForEnvoyTimeout:     getDurationFromEnv("WAIT_FOR_ENVOY_TIMEOUT", time.Duration(0), loggingEnabled),
		IstioQuitAPI:            getStringFromEnv("ISTIO_QUIT_API", "", loggingEnabled),
		NeverKillIstio:          getBoolFromEnv("NEVER_KILL_ISTIO", false, loggingEnabled),
		NeverKillIstioOnFailure: getBoolFromEnv("NEVER_KILL_ISTIO_ON_FAILURE", false, loggingEnabled),
		GenericQuitEndpoints:    getStringArrayFromEnv("GENERIC_QUIT_ENDPOINTS", make([]string, 0), loggingEnabled),
		QuitRequestTimeout:      getDurationFromEnv("QUIT_REQUEST_TIMEOUT", time.Second*5, loggingEnabled),
		QuitWithoutEnvoyTimeout: getDurationFromEnv("QUIT_WITHOUT_ENVOY_TIMEOUT", time.Duration(0), loggingEnabled),
	}

	if config.IstioQuitAPI == "" {
		config.IstioQuitAPI = replacePort(config.EnvoyAdminAPI, defaultAdminAPIPort, defaultQuitAPIPort)
	}

	return config
}

func getStringArrayFromEnv(name string, defaultVal []string, logEnabled bool) []string {
	userValCsv := strings.Trim(os.Getenv(name), " ")

	if userValCsv == "" {
		return defaultVal
	}

	if logEnabled {
		log(fmt.Sprintf("%s: %s", name, userValCsv))
	}

	userValArray := strings.Split(userValCsv, ",")
	if len(userValArray) == 0 {
		return defaultVal
	}

	return userValArray
}

func getStringFromEnv(name string, defaultVal string, logEnabled bool) string {
	userVal := os.Getenv(name)
	if logEnabled {
		log(fmt.Sprintf("%s: %s", name, userVal))
	}
	if userVal != "" {
		return userVal
	}
	return defaultVal
}

func getBoolFromEnv(name string, defaultVal bool, logEnabled bool) bool {
	userVal := os.Getenv(name)
	// User did not set anything return default
	if userVal == "" {
		return defaultVal
	}

	// User set something, check it is valid
	if userVal != "true" && userVal != "false" {
		if logEnabled {
			log(fmt.Sprintf("%s: %s (Invalid value will be ignored)", name, userVal))
		}
		return defaultVal
	}

	// User gave valid option
	if logEnabled {
		log(fmt.Sprintf("%s: %s", name, userVal))
	}
	return userVal == "true"
}

func getDurationFromEnv(name string, defaultVal time.Duration, logEnabled bool) time.Duration {
	userVal := os.Getenv(name)

	// User did not set anything, return default.
	if userVal == "" {
		return defaultVal
	}

	// User has set something, check it is valid.
	if userVal != "" {
		if duration, err := time.ParseDuration(userVal); err == nil {
			// User gave valid option.
			if logEnabled {
				log(fmt.Sprintf("%s: %s", name, userVal))
			}

			return duration
		} else if logEnabled {
			log(fmt.Sprintf("%s: %s (Invalid value will be ignored)", name, userVal))
		}
	}

	return defaultVal
}

// replacePort returns a URL with the port replaced when the sourceURL is valid and has the original port set.
// If the original port does not match or the sourceURL is invalid an empty string is returned.
func replacePort(sourceURL string, original, replacement int) string {
	u, err := url.Parse(sourceURL)
	if err != nil || (u.Port() != fmt.Sprintf("%d", original)) {
		return ""
	}

	u.Host = fmt.Sprintf("%s:%d", u.Hostname(), replacement)
	return u.String()
}
