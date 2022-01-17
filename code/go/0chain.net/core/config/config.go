package config

/*Config - all the config options passed from the command line*/
type Config struct {
	Host            string
	ChainID         string
	SignatureScheme string
	Port            int
	DeploymentMode  byte
	Capacity        int64
}

var Configuration Config
