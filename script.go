package carrot

// A script is an instance of related coroutines running.
// The script will end when the main coroutine ends
// or is cancelled.
type Script struct {
	mainInvoker *invoker
}

// Creates a new script, and starts the mainCoroutine
// in a new thread.
func Start(mainCoroutine func(Invoker)) *Script {
	script := &Script{
		mainInvoker: newInvoker(),
	}
	script.mainInvoker.initialize(mainCoroutine)
	script.mainInvoker.script = script

	return script
}

// Creates a new dead script with no coroutine.
// To be used with script.Change(otherCoroutine)
// or in.Transition(otherCoroutine)
func Create() *Script {
	script := &Script{
		mainInvoker: newInvoker(),
	}
	script.mainInvoker.initialize(nil)
	script.mainInvoker.script = script

	return script
}

// Update causes blocking calls to Yield(), Delay(), DelayAsync() and RunOnUpdate()
// to advance one step. Update is normally called repeatedly inside a loop,
// for instance a game loop, or any application loop in the main thread.
//
// Note: Update is blocking, and will not return until
// a Yield() is called inside the coroutine.
func (script *Script) Update() {
	script.mainInvoker.update()
}

// Changes the current coroutine to a new one. The old
// one is cancelled first before running the new coroutine.
// This is conceptually equivalent to transitions in
// finite state machines.
func (script *Script) Transition(newCoroutine Coroutine) {
	script.mainInvoker.Transition(newCoroutine)
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
	return script.mainInvoker.IsDone()
}

// Use for debugging. Call SetLogging(true) to enable.
func (script *Script) Logf(format string, args ...any) {
	logFn(script, format, args...)
}
