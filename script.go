package carrot

import (
	"sync"
	"time"

	"github.com/nvlled/quest"
)

type ScriptState = uint32

const (
	ScriptStateInit    ScriptState = 0
	ScriptStateStopped ScriptState = 1
	ScriptStateRunning ScriptState = 3
)

type ScriptAction = uint32

const (
	ScriptActionNone    ScriptAction = 0
	ScriptActionCancel  ScriptAction = 1
	ScriptActionRestart ScriptAction = 2
)

// A script is an instance of related coroutines running.
// The script will end when the main coroutine ends
// or is cancelled.
type Script struct {
	mainInvoker *invoker

	mainCoroutine Coroutine
	nextCoroutine Coroutine

	restartTask quest.VoidTask

	state        uint32
	queuedAction uint32

	lastUpdate time.Time

	mu sync.Mutex
}

// Creates a new script, and starts the mainCoroutine
// in a new thread.
func Start(mainCoroutine func(Invoker)) *Script {
	script := &Script{
		mainInvoker: newInvoker(),
		restartTask: quest.NewVoidTask(),
		//bgInvoker:     newInvoker(),
		mainCoroutine: mainCoroutine,
	}
	script.mainInvoker.script = script
	go script.loopRunner()

	script.mu.Lock()
	script.mainInvoker.applyCancel()
	script.restartTask.Resolve(None)
	script.mu.Unlock()

	return script
}

// Creates a new dead script with no coroutine.
// To be used with script.Change(otherCoroutine)
// or in.Transition(otherCoroutine)
func Create() *Script {
	script := &Script{
		mainInvoker:   newInvoker(),
		restartTask:   quest.NewVoidTask(),
		mainCoroutine: nil,
	}
	script.mainInvoker.script = script
	go script.loopRunner()

	return script
}

func (script *Script) loopRunner() {
	for {
		script.restartTask.Anticipate()
		script.startCoroutine()

		script.mu.Lock()
		script.state = ScriptStateStopped
		script.mainInvoker.yieldTask.Cancel()
		script.restartTask.Reset()
		script.mu.Unlock()
	}
}

func (script *Script) startCoroutine() {
	defer CatchCancellation()

	script.mu.Lock()
	script.mainInvoker.reset()
	script.mainInvoker.hasYield.Store(true)
	script.mainInvoker.yieldTask.Resolve(None)
	script.state = ScriptStateRunning
	script.mu.Unlock()

	script.mainInvoker.updateTask.Await()

	script.mu.Lock()
	script.mainInvoker.updateTask.Reset()
	script.mainInvoker.hasYield.Store(false)
	script.state = ScriptStateRunning
	script.mu.Unlock()

	script.mainCoroutine(script.mainInvoker)
}

// Changes the current coroutine to a new one. The old
// one is cancelled first before running the new coroutine.
// This is conceptually equivalent to transitions in
// finite state machines.
func (script *Script) Transition(newCoroutine Coroutine) {
	script.mu.Lock()
	script.nextCoroutine = newCoroutine
	script.nextAction(ScriptActionRestart)
	script.mu.Unlock()
}

// Restarts the script. If script is still running,
// the it is Cancel()'ed first, then mainCoroutine
// is started in a new goroutine.
func (script *Script) Restart() {
	script.mu.Lock()
	script.nextAction(ScriptActionRestart)
	script.mu.Unlock()
}

// Cancels the script. All coroutines started inside
// the script will be cancelled.
func (script *Script) Cancel() {
	script.mu.Lock()
	script.nextAction(ScriptActionCancel)
	script.mu.Unlock()
}

// Returns true if the mainCoroutine finishes running.
func (script *Script) IsDone() bool {
	return script.state == ScriptStateStopped
}

// Update causes blocking calls to Yield(), Delay(), DelayAsync() and RunOnUpdate()
// to advance one step. Update is normally called repeatedly inside a loop,
// for instance a game loop, or any application loop in the main thread.
//
// Note: Update is blocking, and will not return until
// a Yield() is called inside the coroutine.
func (script *Script) Update() {
	none := ScriptActionNone
	if script.queuedAction != none {
		script.mu.Lock()
		if script.queuedAction == ScriptActionCancel {
			script.applyCancel()
			script.queuedAction = none
		} else if script.queuedAction == ScriptActionRestart {
			if script.applyRestart() {
				script.queuedAction = none
			}
		}
		script.mu.Unlock()
	}

	script.mainInvoker.update()
}

func (script *Script) applyCancel() {
	if script.state == ScriptStateRunning {
		script.mainInvoker.applyCancel()
	}
}

func (script *Script) applyRestart() bool {
	if script.state == ScriptStateRunning {
		script.mainInvoker.applyCancel()
		script.mainInvoker.reset()
		return false
	}

	if script.nextCoroutine != nil {
		script.mainCoroutine = script.nextCoroutine
		script.nextCoroutine = nil
	}
	script.restartTask.Resolve(None)
	script.mainInvoker.yieldTask.Resolve(None)
	return true
}

func (script *Script) nextAction(action ScriptAction) {
	if script.queuedAction < action {
		script.queuedAction = action
	}
}
