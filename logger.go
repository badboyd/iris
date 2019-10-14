package main

import (
	"os"
	"strings"

	"go.uber.org/zap"
)

// getLogger returns a *zap.SugaredLogger
func getLogger(module string) *zap.SugaredLogger {
	logLevel := os.Getenv("LOG_LEVEL")
	upperModule := strings.ToUpper(module)
	if os.Getenv("LOG_LEVEL_"+upperModule) != "" {
		logLevel = os.Getenv("LOG_LEVEL_" + upperModule)
	}

	runEnv := os.Getenv("RUN_ENV")
	var config zap.Config
	if strings.ToUpper(runEnv) == "DEV" {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}

	config.Level.UnmarshalText([]byte(logLevel))
	log, _ := config.Build()

	return log.Named(module).Sugar()
}
