// Description: This file contains the struct definitions for the configuration file.
package pkg

// Config - represents the configuration file
type Config struct {
	WebServer WebServer `yaml:"web-server"`
   	Logging Logging `yaml:"logging"`    
}

// 
type WebServer struct {
	Port     string `yaml:"port"`
	Protocol string `yaml:"protocol"`
	SSLCert  string `yaml:"ssl_cert_file,omitempty"`
	SSLKey   string `yaml:"ssl_key_file,omitempty"`
	BaseDir  string `yaml:"base_dir"`
}

// Logging - represents the logging configuration
type Logging struct {
	LogFile string `yaml:"log_file"`
	LogSeverity string `yaml:"log_severity"`
	LogMaxSize int `yaml:"log_max_size"`
	LogMaxFiles int `yaml:"log_max_files"`
	LogMaxAge int `yaml:"log_max_age"`
}