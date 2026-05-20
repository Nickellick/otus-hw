package hw06pipelineexecution

type (
	In  = <-chan interface{}
	Out = In
	Bi  = chan interface{}
)

type Stage func(in In) (out Out)

func ExecutePipeline(in In, done In, stages ...Stage) Out {
	out := in

	for _, stage := range stages {
		out = stage(orDone(out, done))
	}

	return orDone(out, done)
}

func orDone(in, done In) In {
	orDoneCh := make(Bi)

	closeFunc := func() {
		close(orDoneCh)
		for v := range in {
			_ = v
		}
	}

	go func() {
		defer closeFunc()
		for {
			select {
			case <-done:
				return
			case v, ok := <-in:
				if !ok {
					return
				}
				select {
				case <-done:
					return
				case orDoneCh <- v:
				}
			}
		}
	}()
	return orDoneCh
}
