package anka

import (
	"os"

	"github.com/sirupsen/logrus"
)

type Option func(*provider)

func WithUsername(username string) Option {
	return func(p *provider) {
		p.username = username
	}
}

func WithPassword(password string) Option {
	return func(p *provider) {
		p.password = password
	}
}

func WithVMID(vmID string) Option {
	return func(p *provider) {
		p.vmID = vmID
	}
}

// WithUserData returns an option to set the cloud-init template from a file location or passed in text.
func WithUserData(text, path string) Option {
	if text != "" {
		return func(p *provider) {
			p.userData = text
		}
	} else {
		return func(p *provider) {
			if path != "" {
				data, err := os.ReadFile(path)
				if err != nil {
					logrus.WithError(err).
						Fatalln("failed to read user_data file")
					return
				}
				p.userData = string(data)
			}
		}
	}
}

// WithRootDirectory sets the root directory for the virtual machine.
func WithRootDirectory(dir string) Option {
	return func(p *provider) {
		p.rootDir = tempdir(dir)
	}
}
