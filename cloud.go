package jerm

type CloudStorage interface {
	Delete(string) error
	Upload(string) error
	Accessible() error
	CreateBucket(config bool) error
}

type CloudMonitor interface {
	Watch()
	Clear(string) error
}

type CloudPlatform interface {
	Deploy(string) (bool, error)
	Update(string) error
	Undeploy() error
	Build() (string, error)
	Rollback(int) error
	Logs()
	Invoke(string) error
	Cert() error
}
