package jerm

type CloudStorage interface {
	Delete(string) error
	Upload(string) error
}

type CloudMonitor interface {
	Monitor()
	DeleteLog()
}

type CloudPlatform interface {
	Deploy(string) (bool, error)
	Update(string) error
	Undeploy() error
	Build() (string, error)
	Rollback() error
	Logs()
	Invoke(string) error
}
