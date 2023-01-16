package carrot

// katana is used to simulate coroutine behaviour.
// Consider the following:
// | main thread            | coroutine
// | -----------------------|-------------------
// | Start()                |
// | 	                    | loopRunner()
// | 	                    |  YieldRight() //1
// | 	                    |  running = true
// | Update()               |  coroutine() // enter coroutine
// |  YieldLeft() //1       |   println("a")
// | Update()               |   YieldRight() // 2
// |  YieldLeft() //2       |   println("b")
// | Update()               |  // exit coroutine
// |  YieldLeft() //3       |  running = false
// | 	                    |  // next loop iteration if restarted
// | 	                    |  YieldRight() // 3
//
// Note each yield has a matching number.
// The first YieldLeft() blocks, and then unblocks
// the first YieldRight(), which causes the coroutine
// to start.
// The second YieldRight() suspends the coroutine,
// and resumes control on the main thread.
// The output "a\n" and "b\n" will be printed on a separate game loop.
// It's called katana because one of the following holds:
// - why not
// - I'm bad at naming
// - it sounds like wielding something left and right
// - it's an abstract concept that has no actual analogue
// - am weebo
type katana struct {
	c chan Void
}

func newKatana() *katana {
	return &katana{
		c: make(chan Void),
	}
}

// Yields control from the main thread
// to the coroutine. It will not return
// until YieldRight() is called.
func (k *katana) YieldLeft() {
	k.c <- None
	<-k.c
}

// Yields control from the coroutine
// to the main thread. It will not return
// until YieldLeft() is called.
func (k *katana) YieldRight() {
	<-k.c
	k.c <- None
}
