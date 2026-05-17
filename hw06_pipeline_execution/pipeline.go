package hw06pipelineexecution

type (
	In  = <-chan interface{}
	Out = In
	Bi  = chan interface{}
)

type Stage func(in In) (out Out)

func ExecutePipeline(in In, done In, stages ...Stage) Out {
	if len(stages) == 0 {
		return in
	}

	out := stages[0](in)
	for _, stage := range stages[1:] {
		select {
		case <-done:
			return out
		default:
			out = stage(out)
		}
	}

	return out
}
