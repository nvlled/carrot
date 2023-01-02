package carrot

// A script is an instance of related coroutines running.
// The script will end when the main coroutine ends
// or is cancelled.
type Script struct {
	mainCoroutine Coroutine
	nextCoroutine Coroutine

	mainInvoker *invoker
	done        bool
}

// Creates a new script, and starts the mainCoroutine
// in a new thread.
func Start(mainCoroutine func(Invoker)) *Script {
	script := &Script{
		done:          false,
		mainInvoker:   newInvoker(),
		mainCoroutine: mainCoroutine,
	}
	script.mainInvoker.script = script
	script.mainInvoker.endTask.Resolve(None)
	go script.start()

	return script
}

// Creates a new dead script with no coroutine.
// To be used with script.Change(otherCoroutine)
// or in.Transition(otherCoroutine)
func Create() *Script {
	script := &Script{
		done:          true,
		mainInvoker:   newInvoker(),
		mainCoroutine: nil,
	}
	script.mainInvoker.script = script

	return script
}

func (script *Script) start() {
	if script.mainCoroutine == nil {
		return
	}

	defer func() {
		script.done = true
		script.mainInvoker.yieldTask.Resolve(None)
		script.mainInvoker.endTask.Resolve(None)
	}()
	defer CatchCancellation()

	script.done = false
	script.startCoroutine()
	script.mainInvoker.Cancel()
}

func (script *Script) startCoroutine() {
	script.mainInvoker.Yield()
	script.mainCoroutine(script.mainInvoker)
}

// Changes the current coroutine to a new one. The old
// one is cancelled first before running the new coroutine.
// This is conceptually equivalent to transitions in
// finite state machines.
func (script *Script) Transition(newCoroutine Coroutine) {
	script.nextCoroutine = newCoroutine
}

// Restarts the script. If script is still running,
// the it is Cancel()'ed first, then mainCoroutine
// is started in a new goroutine.
func (script *Script) Restart() {
	if !script.done {
		script.Cancel()
		script.mainInvoker.endTask.Anticipate()
	}

	script.mainInvoker.reset()
	//script.mainInvoker = newInvoker()
	go script.start()
}

// Returns true if the mainCoroutine finishes running.
func (script *Script) IsDone() bool {
	return script.done
}

// Update causes blocking calls to Yield(), Delay(), DelayAsync() and RunOnUpdate()
// to advance one step. Update is normally called repeatedly inside a loop,
// for instance a game loop, or any application loop in the main thread.
//
// Note: Update is blocking, and will not return until
// a Yield() is called inside the coroutine.
func (script *Script) Update() {
	if script.nextCoroutine != nil {
		script.Cancel()
		script.mainCoroutine = script.nextCoroutine
		script.nextCoroutine = nil
		script.Restart()
	} else if !script.done {
		script.mainInvoker.update()
	}
}

// Cancels the script. All coroutines started inside
// the script will be cancelled.
func (script *Script) Cancel() {
	script.mainInvoker.applyCancel()
}
