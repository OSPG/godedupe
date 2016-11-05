package main

// This file is under development, work not finished

// WorkUnit execute an action
type WorkUnit struct {
}

// RoutinePool execute go routines
type RoutinePool struct {
	routineCounter int      // total routines created in this pool
	shutdown       chan int // channel to shutdown the pool
}

// Create the pool
func Create() *RoutinePool {
	pool := RoutinePool{}
	pool.loop()
	return &pool
}

// main loop for the pool that listen for signals. Manage the pool lifecycle
func (pool *RoutinePool) loop() {

}

// Submit to the pool a WorkUnit to be executed
func (pool *RoutinePool) Submit(work *WorkUnit) {

}

// Shutdown close the pool when all WorkerUnit finish
func (pool *RoutinePool) Shutdown() {
	close(pool.shutdown)
}
