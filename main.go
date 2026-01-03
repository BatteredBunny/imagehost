package main

import (
	"github.com/BatteredBunny/hostling/cmd"
)

func main() {
	app := cmd.InitializeApplication()

	app.StartJobScheudler()

	app.Run()
}
