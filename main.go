package main

import (
	"github.com/BatteredBunny/imagehost/cmd"
)

func main() {
	app := cmd.InitializeApplication()

	go app.AutoDeletion()

	app.Run()
}
