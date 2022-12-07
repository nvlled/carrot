package carrot

import (
	"github.com/nvlled/mud"
)

var insectHive = mud.NewPool()

// Pre-allocate a number of insects of the given type.
func BreedMore(count int) {
	mud.PreAlloc(insectHive, newInsect, count)
}

// Allocate a insect using an object pool.
// Free the insect afterwards with Free().
// Use only when gc is a concern.
func SpawnInsect() Insect {
	insect := mud.Alloc(insectHive, newInsect)
	return insect
}

// Free a insect that was previously Alloc()'d.
func ReleaseInsect(insect Insect) {
	object, ok := insect.(*insectoid)
	if !ok {
		return
	}
	mud.Free(insectHive, object)
}

func spawnInsectoid() *insectoid {
	insect := mud.Alloc(insectHive, newInsect)
	return insect
}
func releaseInsectoid(insect *insectoid) {
	mud.Free(insectHive, insect)
}
