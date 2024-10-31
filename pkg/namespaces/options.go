package namespaces

// Option is a function that sets an optional configuration.
type Option func(*Configuration)

// Configuration holds the runtime configuration for this package.
type Configuration struct {
	// Logf is a function that will be used to log messages. If not
	// provided the default logger will be used.
	Logf func(string, ...interface{})
	// Port is the port to use for the UDP and TCP pings.
	Port int
}

// NewConfiguration creates a new configuration with the provided options.
func NewConfiguration(options ...Option) Configuration {
	cfg := Configuration{
		Logf: func(string, ...interface{}) {},
		Port: 8080,
	}
	for _, o := range options {
		o(&cfg)
	}
	return cfg
}

// WithLogf sets the log function for this package.
func WithLogf(f func(string, ...interface{})) Option {
	return func(c *Configuration) {
		c.Logf = f
	}
}

// WithPort sets the port to use for the UDP and TCP pings.
func WithPort(port int) Option {
	return func(c *Configuration) {
		c.Port = port
	}
}
