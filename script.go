package carrot

// A Script is an instance of related coroutines running.
type Script struct {
	baseControl *Control
}

// Creates a new coroutine script. Coroutine will only start
// on the first call to Update().
func Start(coroutine Coroutine) *Script {
	script := &Script{
		baseControl: NewControl(),
	}
	script.baseControl.initialize(coroutine)

	return script
}

// Creates an inactive coroutine script.
// To be used with script.Transition(otherCoroutine).
func Create() *Script {
	script := &Script{
		baseControl: NewControl(),
	}
	script.baseControl.initialize(nil)

	return script
}

// Update causes blocking calls to Yield(), Delay(), DelayAsync() and RunOnUpdate()
// to advance one step. Update is normally called repeatedly inside a loop,
// for instance a game loop, or any application loop in the main thread.
//
//	Note: Update is blocking, and will not return until
//	a Yield() is called inside the coroutine.
func (script *Script) Update() {
	script.baseControl.update()
}

// Changes the current coroutine function to a new one. The old
// one is cancelled first before running the new coroutine.
// This is conceptually equivalent to transitions in
// finite state machines.
func (script *Script) Transition(newCoroutine Coroutine) {
	script.baseControl.Transition(newCoroutine)
}

// Restarts the coroutine. If the coroutine is still running,
// it is Cancel()'ed first, then the coroutine
// is started again.
//
//	Note: restart will be done in the next Update()
func (script *Script) Restart() {
	script.baseControl.Restart()
}

// Cancels the coroutine. All coroutines started inside
// the script will be cancelled.
//
//	Note: cancellation will be done in the next Update()
func (script *Script) Cancel() {
	script.baseControl.Cancel()
}

// Returns true if the coroutine finishes running
// and is not restarting.
func (script *Script) IsDone() bool {
	return script.baseControl.IsDone()
}

// Use for debugging. Call SetLogging(true) to enable.
func (script *Script) Logf(format string, args ...any) {
	logFn(script.baseControl, format, args...)
}
