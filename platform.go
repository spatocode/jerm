package jerm

type Platform interface {
	Deploy(string) (bool, error)
	Update(string) error
	Undeploy() error
	Build() (string, error)
	Rollback() error
	CreateFunctionEntry(string) error
	Logs()
}
