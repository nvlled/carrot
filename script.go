package carrot

import (
	bits "github.com/nvlled/carrot/atombits"
)

// A script is an instance of related coroutines running.
// The script will end when the main coroutine ends
// or is cancelled.
type Script struct {
	mainInvoker *invoker
	// TODO: move to invoker
	mainCoroutine Coroutine
}

// Creates a new script, and starts the mainCoroutine
// in a new thread.
func Start(mainCoroutine func(Invoker)) *Script {
	script := &Script{
		mainInvoker:   newInvoker(),
		mainCoroutine: mainCoroutine,
	}
	script.Logf("created")
	script.mainInvoker.script = script
	script.Restart()
	go script.loopRunner()

	return script
}

// Creates a new dead script with no coroutine.
// To be used with script.Change(otherCoroutine)
// or in.Transition(otherCoroutine)
func Create() *Script {
	script := &Script{
		mainInvoker:   newInvoker(),
		mainCoroutine: nil,
	}
	script.Logf("created")
	script.mainInvoker.script = script
	go script.loopRunner()

	return script
}

// Update causes blocking calls to Yield(), Delay(), DelayAsync() and RunOnUpdate()
// to advance one step. Update is normally called repeatedly inside a loop,
// for instance a game loop, or any application loop in the main thread.
//
// Note: Update is blocking, and will not return until
// a Yield() is called inside the coroutine.
func (script *Script) Update() {
	in := script.mainInvoker
	restartNow := in.isRestarting()
	if in.isCancelling() {
		in.applyCancel()
		restartNow = false
	} else if restartNow {
		bits.Unset(&in.action, ActionRestart)
		in.applyRestart()
	}

	if script.mainCoroutine != nil && (in.IsRunning() || restartNow) {
		in.kanata.YieldLeft()
	}
}

func (script *Script) loopRunner() {
	in := script.mainInvoker
	in.setRunning(true)
	for {
		script.Logf("loop start")
		script.mainInvoker.kanata.YieldRight()

		script.Logf("coroutine start")
		in.setRunning(true)
		script.startCoroutine()

		script.Logf("coroutine end")
		in.setRunning(false)
	}
}

func (script *Script) startCoroutine() {
	defer catchCancellation()
	script.mainCoroutine(script.mainInvoker)
}

// Changes the current coroutine to a new one. The old
// one is cancelled first before running the new coroutine.
// This is conceptually equivalent to transitions in
// finite state machines.
func (script *Script) Transition(newCoroutine Coroutine) {
	script.mainCoroutine = newCoroutine
	script.mainInvoker.Restart()
	script.mainInvoker.Cancel()
}

// Restarts the script. If script is still running,
// it is Cancel()'ed first, then the coroutine
// is started again.
// Note: restart will be done in the next Update()
func (script *Script) Restart() {
	script.mainInvoker.Restart()
}

// Cancels the script. All coroutines started inside
// the script will be cancelled.
// Note: cancellation will be done in the next Update()
func (script *Script) Cancel() {
	script.mainInvoker.Cancel()
}

// Returns true if the main coroutine finishes running
// and is not restarting.
func (script *Script) IsDone() bool {
	in := script.mainInvoker
	return !in.IsRunning() && !in.isRestarting()
}

// Use for debugging. Call SetLogging(true) to enable.
func (script *Script) Logf(format string, args ...any) {
	logFn(script, format, args...)
}
