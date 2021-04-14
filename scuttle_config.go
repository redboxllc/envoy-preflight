package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// ScuttleConfig ... represents Scuttle's configuration based on environment variables or defaults.
type ScuttleConfig struct {
	LoggingEnabled           bool
	Mode				     string
	ReadinessEndpoints		 []string
	ReadinessMaxPollTime	 time.Duration
	ReadinessTimeout		 time.Duration
	StopEndpoints			 []string
	StopSkipOnFailure   	 bool

	// Deprecated
	EnvoyAdminAPI           string
	StartWithoutEnvoy       bool
	WaitForEnvoyTimeout     time.Duration
	IstioQuitAPI            string
	NeverKillIstio          bool
	IstioFallbackPkill      bool
	NeverKillIstioOnFailure bool
	GenericQuitEndpoints    []string
	QuitWithoutEnvoyTimeout time.Duration
}

func log(message string) {
	if config.LoggingEnabled {
		fmt.Printf("%s scuttle: %s\n", time.Now().UTC().Format("2006-01-02T15:04:05Z"), message)
	}
}

// Gets the ScuttleConfig, first using any defined configuration modes
// If no configuration mode is set, Environment Variables are used instead
func getConfig() ScuttleConfig {
	// Create base config, which will be overriden by a mode and explicitly
	// set env vars later
	loggingEnabled := getBoolFromEnv("SCUTTLE_LOGGING", true, false)
	config := ScuttleConfig{
		LoggingEnabled: loggingEnabled,
	}

	mode := strings.ToLower(strings.Trim(getStringFromEnv("SCUTTLE_MODE", "", loggingEnabled), " "))
	switch mode {
	case "istio":
		config.SetIstioDefaults()
		config.SetEnvVars()
	case "":
		config.SetEnvVars()
	default:
		panic(fmt.Errorf("Provided Scuttle Mode is invalid: %s", mode))
	}

	return config
}

// Overrides the provided baseConfig with Istio defaults
func (c ScuttleConfig) SetIstioDefaults() {
	c.ReadinessEndpoints = []string{"http://127.0.0.1:15020/healthz/ready"}
	c.StopEndpoints = []string{"http://127.0.0.1:15020/quitquitquit"}
}

// Overrides the provided baseConfig with provided Environment Variable values
func (c ScuttleConfig) SetEnvVars() {
	c.ReadinessEndpoints = getStringArrayFromEnv( "SCUTTLE_READINESS_ENDPOINTS", c.ReadinessEndpoints, c.LoggingEnabled)
	c.ReadinessMaxPollTime = getDurationFromEnv(  "SCUTTLE_READINESS_MAX_POLL_TIME", c.ReadinessMaxPollTime, c.LoggingEnabled)
	c.ReadinessTimeout = getDurationFromEnv(      "SCUTTLE_READINESS_TIMEOUT", c.ReadinessTimeout, c.LoggingEnabled)
	c.StopEndpoints = getStringArrayFromEnv( 	  "SCUTTLE_STOP_ENDPOINTS", c.StopEndpoints, c.LoggingEnabled)
	c.StopSkipOnFailure = getBoolFromEnv(         "SCUTTLE_STOP_SKIP_ON_FAILURE", c.StopSkipOnFailure, c.LoggingEnabled)
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
