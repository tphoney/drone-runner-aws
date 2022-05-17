package amazon

import (
	"os"

	"github.com/drone-runners/drone-runner-aws/oshelp"

	"github.com/sirupsen/logrus"
)

type Option func(*provider)

func WithAccessKeyID(accessKeyID string) Option {
	return func(p *provider) {
		p.accessKeyID = accessKeyID
	}
}

// WithSecretAccessKey sets the AWS secret access key.
func WithSecretAccessKey(secretAccessKey string) Option {
	return func(p *provider) {
		p.secretAccessKey = secretAccessKey
	}
}

// WithRootDirectory sets the root directory for the virtual machine.
func WithRootDirectory(dir string) Option {
	return func(p *provider) {
		p.rootDir = tempdir(dir)
	}
}

// WithDeviceName returns an option to set the device name.
func WithDeviceName(n string) Option {
	return func(p *provider) {
		p.deviceName = n
	}
}

// WithAMI returns an option to set the image.
func WithAMI(ami string) Option {
	return func(p *provider) {
		p.image = ami
	}
}

// WithPrivateIP returns an option to set the private IP address.
func WithPrivateIP(private bool) Option {
	return func(p *provider) {
		p.allocPublicIP = !private
	}
}

// WithRetries returns an option to set the retry count.
func WithRetries(retries int) Option {
	return func(p *provider) {
		p.retries = retries
	}
}

// WithRegion returns an option to set the target region.
func WithRegion(region string) Option {
	return func(p *provider) {
		p.region = region
	}
}

// WithSecurityGroup returns an option to set the instance size.
func WithSecurityGroup(group ...string) Option {
	return func(p *provider) {
		p.groups = group
	}
}

// WithSize returns an option to set the instance size.
func WithSize(size, arch string) Option {
	return func(p *provider) {
		p.size = size
		// set default instance type if not provided
		if p.size == "" {
			if arch == "arm64" {
				p.size = "a1.medium"
			} else {
				p.size = "t3.nano"
			}
		}
	}
}

// WithSizeAlt returns an option to set the alternate instance
// size. If instance creation fails, the system will attempt to
// provision a second instance using the alternate size.
func WithSizeAlt(size string) Option {
	return func(p *provider) {
		p.sizeAlt = size
	}
}

// WithSubnet returns an option to set the subnet id.
func WithSubnet(id string) Option {
	return func(p *provider) {
		p.subnet = id
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

// WithVolumeSize returns an option to set the volume size
// in gigabytes.
func WithVolumeSize(s int64) Option {
	return func(p *provider) {
		p.volumeSize = s
	}
}

// WithVolumeType returns an option to set the volume type.
func WithVolumeType(t string) Option {
	return func(p *provider) {
		p.volumeType = t
	}
}

// WithVolumeIops returns an option to set the volume iops.
func WithVolumeIops(i int64) Option {
	return func(p *provider) {
		p.volumeIops = i
	}
}

// WithIamProfileArn returns an option to set the iam profile arn.
func WithIamProfileArn(t string) Option {
	return func(p *provider) {
		p.iamProfileArn = t
	}
}

// WithVpc returns an option to set the vpc.
func WithVpc(t string) Option {
	return func(p *provider) {
		p.vpc = t
	}
}

// WithMarketType returns an option to set the instance market type.
func WithMarketType(t string) Option {
	return func(p *provider) {
		p.spotInstance = t == "spot"
	}
}

// WithZone returns an option to set the zone.
func WithZone(zone string) Option {
	return func(p *provider) {
		p.availabilityZone = zone
	}
}

// WithKeyPair returns an option to set the key pair.
func WithKeyPair(keyPair string) Option {
	return func(p *provider) {
		p.keyPairName = keyPair
	}
}

func WithHibernate(hibernate bool) Option {
	return func(p *provider) {
		p.hibernate = hibernate
	}
}

func WithUser(user, platform string) Option {
	return func(p *provider) {
		p.user = user
		// set the default ssh user. this user account is responsible for executing the pipeline script.
		if p.user == "" {
			if platform == oshelp.OSWindows {
				p.user = "Administrator"
			} else {
				p.user = "root"
			}
		}
	}
}
