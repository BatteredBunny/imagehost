//go:build wireinject
// +build wireinject

package cmd

import (
	"github.com/google/wire"
)

type uninitializedApplication Application

func InitializeApplication() *Application {
	panic(wire.Build(wire.NewSet(
		setupLogging,
		initializeConfig,
		setupRatelimiting,
		prepareDB,
		prepareStorage,

		wire.Struct(
			new(uninitializedApplication),
			"Logger",
			"config",
			"db",
			"s3client",
			"RateLimiter",
		),

		setupRouter, // Finishes the setup
	)))
}
