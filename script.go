package carrot

// A script is an instance of related coroutines running.
// The script will end when the main coroutine ends
// or is cancelled.
type Script struct {
	mainCoroutine Coroutine
	mainInvoker   *invoker
	done          bool
}

// Creates a new script, and starts the mainCoroutine
// in a new thread.
func Start(mainCoroutine func(Invoker)) *Script {
	script := &Script{
		mainCoroutine: mainCoroutine,
		mainInvoker:   newInvoker(),
		done:          false,
	}
	go script.start()

	return script
}

func (script *Script) start() {
	defer func() { script.done = true }()
	defer CatchCancellation()

	script.done = false
	script.mainCoroutine(script.mainInvoker)
	script.mainInvoker.Cancel()
}

// Restarts the script. If script is still running,
// the it is Cancel()'ed first, then mainCoroutine
// is started in a new thread.
func (script *Script) Restart() {
	if !script.done {
		script.Cancel()
	}
	go script.start()
}

// Returns true if the mainCoroutine finishes running.
func (script *Script) IsDone() bool {
	return script.done
}

// Update causes blocking calls to Yield(), Delay(), DelayAsync() and RunOnUpdate()
// to advance one step. Update is normally called repeatedly inside a loop,
// for instance a game loop, or any application loop in the main thread.
func (script *Script) Update() {
	script.mainInvoker.update()
}

// Cancels the script. All coroutines started inside
// the script will be cancelled.
func (script *Script) Cancel() {
	script.mainInvoker.Cancel()
}
