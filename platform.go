package jerm

type CloudStorage interface {
	delete(string) error
	upload(string) error
}

type CloudMonitor interface {
	monitor()
}

type Platform interface {
	Deploy(string) (bool, error)
	Update(string) error
	Undeploy() error
	Build() (string, error)
	Rollback() error
	Logs()
}
