package config

import "fmt"

type Config struct {
	Owner string
	Repo  string
}

func (c Config) RepoNWO() string {
	return fmt.Sprintf("%s/%s", c.Owner, c.Repo)
}

func (c Config) Validate() error {
	if c.Owner == "" || c.Repo == "" {
		return fmt.Errorf("owner and repo are required (use -R owner/repo)")
	}
	return nil
}
