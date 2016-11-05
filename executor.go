package main

// WorkUnit execute an action
type WorkUnit struct {
}

// RoutinePool execute go routines
type RoutinePool struct {
	routineCounter int // total routines created in this pool
}

// Create the pool
func Create() *RoutinePool {
	pool := RoutinePool{}
	return &pool
}

// Submit to the pool a WorkUnit to be executed
func (pool *RoutinePool) Submit(work *WorkUnit) {

}
