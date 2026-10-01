package main

import (
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/unit"

	"ircgram/internal/ui"
)

func main() {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("IRCgram"), app.Size(unit.Dp(1000), unit.Dp(700)))
		if err := ui.Run(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}
