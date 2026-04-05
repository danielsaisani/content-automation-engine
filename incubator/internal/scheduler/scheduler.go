type Scheduler interface {
	Run() error
}

type RealScheduler struct {
	clock Clock
}

func NewRealScheduler(clock Clock) *RealScheduler {
	return &RealScheduler{clock: clock}
}

func (s *RealScheduler) Run() error {
	return nil
}
