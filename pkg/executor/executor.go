package executor

type Event struct {
	Source string
}

type Executor interface {
	OnWatchEvent(ev Event) error
	Start() error
	Stop() error
}
