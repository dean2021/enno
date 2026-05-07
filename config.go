package enno

const DefaultMaxToolRounds = 50

type Config struct {
	Provider      Provider
	SystemPrompt  string
	Tools         []Tool
	MaxToolRounds int
	EventHandler  EventHandler
}

func (c Config) withDefaults() Config {
	if c.MaxToolRounds <= 0 {
		c.MaxToolRounds = DefaultMaxToolRounds
	}
	return c
}
