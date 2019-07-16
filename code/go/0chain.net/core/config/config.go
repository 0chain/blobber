package config

/*Config - all the config options passed from the command line*/
type Config struct {
	Host           string
	Port           int
	ChainID        string
	Capacity       int64
	DeploymentMode byte
}

var Configuration Config
