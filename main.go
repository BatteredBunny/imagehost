package main

import (
	"github.com/BatteredBunny/imagehost/cmd"
)

func main() {
	app := cmd.InitializeApplication()

	app.StartJobScheudler()

	app.Run()
}
