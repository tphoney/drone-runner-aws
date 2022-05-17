package google

import (
	"os"

	"github.com/drone-runners/drone-runner-aws/oshelp"

	"github.com/sirupsen/logrus"
)

type Option func(*provider)

// WithDiskSize returns an option to set the instance disk
// size in gigabytes.
func WithDiskSize(diskSize int64) Option {
	return func(p *provider) {
		p.diskSize = diskSize
	}
}

// WithDiskType returns an option to set the instance disk type.
func WithDiskType(diskType string) Option {
	return func(p *provider) {
		p.diskType = diskType
	}
}

// WithMachineImage returns an option to set the image.
func WithMachineImage(image string) Option {
	return func(p *provider) {
		p.image = image
	}
}

// WithMachineType returns an option to set the instance type.
func WithMachineType(size string) Option {
	return func(p *provider) {
		p.size = size
	}
}

// WithNetwork returns an option to set the network.
func WithNetwork(network string) Option {
	return func(p *provider) {
		p.network = network
	}
}

// WithSubnetwork returns an option to set the subnetwork.
func WithSubnetwork(subnetwork string) Option {
	return func(p *provider) {
		p.subnetwork = subnetwork
	}
}

// WithPrivateIP returns an option to set the private IP address.
func WithPrivateIP(private bool) Option {
	return func(p *provider) {
		p.privateIP = private
	}
}

// WithProject returns an option to set the project.
func WithProject(project string) Option {
	return func(p *provider) {
		p.projectID = project
	}
}

// WithJSONPath returns an option to set the json path
func WithJSONPath(path string) Option {
	return func(p *provider) {
		p.JSONPath = path
	}
}

// WithTags returns an option to set the resource tags.
func WithTags(tags ...string) Option {
	return func(p *provider) {
		p.tags = tags
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

// WithUserDataKey allows to set the user data key for Google Cloud Platform
// This allows user to set either user-data or a startup script
func WithUserDataKey(text, platform string) Option {
	return func(p *provider) {
		p.userDataKey = text
		if p.userDataKey == "" && platform == oshelp.OSLinux {
			p.userDataKey = "user-data"
		} else {
			p.userDataKey = "windows-startup-script-ps1"
		}
	}
}

// WithZones WithZone returns an option to set the target zone.
func WithZones(zones ...string) Option {
	return func(p *provider) {
		p.zones = zones
	}
}

// WithScopes returns an option to set the scopes.
func WithScopes(scopes ...string) Option {
	return func(p *provider) {
		p.scopes = scopes
	}
}

// WithServiceAccountEmail returns an option to set the ServiceAccountEmail.
func WithServiceAccountEmail(email string) Option {
	return func(p *provider) {
		p.serviceAccountEmail = email
	}
}
