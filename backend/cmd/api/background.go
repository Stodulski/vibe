package main

import "fmt"

// background runs fn on a tracked goroutine, recovering panics so one failed
// task cannot take the process down, and so a graceful shutdown waits for it.
//
// It stays in the composition root rather than moving to a package: it is
// process lifecycle, and every module that needs it receives it as a
// func(func()) it can call without knowing how goroutines here are tracked.
func (app *application) background(fn func()) {
	app.wg.Go(func() {
		defer func() {
			if err := recover(); err != nil {
				app.logger.Error("background task panic", "error", fmt.Sprintf("%v", err))
			}
		}()

		fn()
	})
}
