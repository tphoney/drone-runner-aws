package poolfile

import (
	"fmt"

	"github.com/drone-runners/drone-runner-aws/command/config"
	"github.com/drone-runners/drone-runner-aws/internal/drivers"
	"github.com/drone-runners/drone-runner-aws/internal/drivers/amazon"
	"github.com/drone-runners/drone-runner-aws/internal/drivers/anka"
	"github.com/drone-runners/drone-runner-aws/internal/drivers/google"
	"github.com/drone-runners/drone-runner-aws/internal/drivers/vmfusion"
	"github.com/drone-runners/drone-runner-aws/oshelp"
	"github.com/drone-runners/drone-runner-aws/types"
	"gopkg.in/yaml.v2"

	"github.com/sirupsen/logrus"
)

func ProcessPool(poolFile *config.PoolFile, runnerName string) ([]drivers.Pool, error) {
	var pools = []drivers.Pool{}

	for _, i := range poolFile.Instances {
		i := i
		switch i.Type {
		case string(types.ProviderVMFusion):
			var v, ok = i.Spec.(*config.VMFusion)
			if !ok {
				logrus.Errorln("unable to parse pool file")
			}
			driver, err := vmfusion.New(
				vmfusion.WithStorePath(v.StorePath),
				vmfusion.WithUsername(v.Account.Username),
				vmfusion.WithPassword(v.Account.Password),
				vmfusion.WithISO(v.ISO),
				vmfusion.WithCPU(v.CPU),
				vmfusion.WithMemory(v.Memory),
				vmfusion.WithVDiskPath(v.VDiskPath),
				vmfusion.WithUserData(v.UserData, v.UserDataPath),
				vmfusion.WithRootDirectory(v.RootDirectory),
			)
			if err != nil {
				logrus.WithError(err).Errorln("unable to create vmfusion config")
			}
			pool := mapPool(&i, runnerName)
			pool.Driver = driver
			pools = append(pools, pool)
		case string(types.ProviderAmazon):
			var a, ok = i.Spec.(*config.Amazon)
			if !ok {
				logrus.Errorln("unable to parse pool file")
			}
			var driver, err = amazon.New(
				amazon.WithAccessKeyID(a.Account.AccessKeyID),
				amazon.WithSecretAccessKey(a.Account.AccessKeySecret),
				amazon.WithZone(a.Account.AvailabilityZone),
				amazon.WithKeyPair(a.Account.KeyPairName),
				amazon.WithDeviceName(a.DeviceName),
				amazon.WithRootDirectory(a.RootDirectory),
				amazon.WithAMI(a.AMI),
				amazon.WithVpc(a.VPC),
				amazon.WithUser(a.User, i.Platform.OS),
				amazon.WithRegion(a.Account.Region),
				amazon.WithRetries(a.Account.Retries),
				amazon.WithPrivateIP(a.Network.PrivateIP),
				amazon.WithSecurityGroup(a.Network.SecurityGroups...),
				amazon.WithSize(a.Size, i.Platform.Arch),
				amazon.WithSizeAlt(a.SizeAlt),
				amazon.WithSubnet(a.Network.SubnetID),
				amazon.WithUserData(a.UserData, a.UserDataPath),
				amazon.WithVolumeSize(a.Disk.Size),
				amazon.WithVolumeType(a.Disk.Type),
				amazon.WithVolumeIops(a.Disk.Iops),
				amazon.WithIamProfileArn(a.IamProfileArn),
				amazon.WithMarketType(a.MarketType),
				amazon.WithHibernate(a.Hibernate),
			)
			if err != nil {
				logrus.WithError(err).Errorln("unable to create google config")
			}
			pool := mapPool(&i, runnerName)
			pool.Driver = driver
			pools = append(pools, pool)
		case string(types.ProviderGoogle):
			var g, ok = i.Spec.(*config.Google)
			if !ok {
				logrus.Errorln("unable to parse pool file")
			}
			var driver, err = google.New(
				google.WithDiskSize(g.Disk.Size),
				google.WithDiskType(g.Disk.Type),
				google.WithMachineImage(g.Image),
				google.WithMachineType(g.MachineType),
				google.WithNetwork(g.Network),
				google.WithSubnetwork(g.Subnetwork),
				google.WithPrivateIP(g.PrivateIP),
				google.WithServiceAccountEmail(g.Account.ServiceAccountEmail),
				google.WithProject(g.Account.ProjectID),
				google.WithJSONPath(g.Account.JSONPath),
				google.WithTags(g.Tags...),
				google.WithScopes(g.Scopes...),
				google.WithUserData(g.UserData, g.UserDataPath),
				google.WithZones(g.Zone...),
				google.WithUserDataKey(g.UserDataKey, i.Platform.OS),
			)
			if err != nil {
				logrus.WithError(err).Errorln("unable to create google config")
			}
			pool := mapPool(&i, runnerName)
			pool.Driver = driver
			pools = append(pools, pool)
		case string(types.ProviderAnka):
			var ak, ok = i.Spec.(*config.Anka)
			if !ok {
				logrus.Errorln("unable to parse pool file")
			}
			driver, err := anka.New(
				anka.WithUsername(ak.Account.Username),
				anka.WithPassword(ak.Account.Password),
				anka.WithRootDirectory(ak.RootDirectory),
				anka.WithUserData(ak.UserData, ak.UserDataPath),
				anka.WithVMID(ak.VMID),
			)
			if err != nil {
				logrus.WithError(err).Errorln("unable to create anka config")
			}
			pool := mapPool(&i, runnerName)
			pool.Driver = driver
			pools = append(pools, pool)
		default:
			return nil, fmt.Errorf("unknown instance type %s", i.Type)
		}
	}
	return pools, nil
}

func mapPool(i *config.Instance, runnerName string) drivers.Pool {
	if i.Pool < 0 {
		i.Pool = 0
	}
	if i.Limit <= 0 {
		i.Limit = 100
	}
	if i.Pool > i.Limit {
		i.Limit = i.Pool
	}
	if i.Platform.OS == "" {
		if i.Type == string(types.ProviderVMFusion) || i.Type == string(types.ProviderAnka) {
			i.Platform.OS = oshelp.OSMac
		} else {
			i.Platform.OS = oshelp.OSLinux
		}
	}
	if i.Platform.Arch == "" {
		i.Platform.Arch = "amd64"
	}
	var pool = drivers.Pool{
		RunnerName: runnerName,
		Name:       i.Name,
		MaxSize:    i.Limit,
		MinSize:    i.Pool,
		OS:         i.Platform.OS,
		Arch:       i.Platform.Arch,
		Version:    i.Platform.Version,
	}
	return pool
}

func ConfigPoolFile(filepath, providerType string, conf *config.EnvConfig) (pool *config.PoolFile, err error) {
	if filepath == "" {
		logrus.Infof("no pool file provided, creating in memmory pool for %s", providerType)
		// generate a pool file
		switch providerType {
		case string(types.ProviderAmazon):
			// do we have the creds?
			if conf.AWS.AccessKeyID == "" || conf.AWS.AccessKeySecret == "" {
				return pool, fmt.Errorf("%s:missing credentials in env variables 'DRONE_AWS_ACCESS_KEY_ID' and 'DRONE_AWS_ACCESS_KEY_SECRET'", providerType)
			}
			return createAmazonPool(conf.AWS.AccessKeyID, conf.AWS.AccessKeySecret, conf.Settings.MinPoolSize, conf.Settings.MaxPoolSize), nil

		default:
			err = fmt.Errorf("unknown provider type %s, unable to create pool file in memory", providerType)
			return pool, err
		}
	}
	pool, err = config.ParseFile(filepath)
	if err != nil {
		logrus.WithError(err).Errorln("exec: unable to parse pool file")
	}
	return pool, err
}

func PrintPoolFile(pool *config.PoolFile) {
	marshalledPool, marshalErr := yaml.Marshal(pool)
	if marshalErr != nil {
		logrus.WithError(marshalErr).
			Errorln("unable to marshal pool file, cannot print")
	}
	fmt.Printf("Pool file:\n%s\n", marshalledPool)
}

func createAmazonPool(accessKeyID, accessKeySecret string, minPoolSize, maxPoolSize int) *config.PoolFile {
	instance := config.Instance{
		Name:    "test_pool",
		Default: true,
		Type:    string(types.ProviderAmazon),
		Pool:    minPoolSize,
		Limit:   maxPoolSize,
		Platform: config.Platform{
			Arch: "amd64",
			OS:   "linux",
		},
		Spec: &config.Amazon{
			Account: config.AmazonAccount{
				Region:          "us-east-2",
				AccessKeyID:     accessKeyID,
				AccessKeySecret: accessKeySecret,
			},
			AMI:  "ami-051197ce9cbb023ea",
			Size: "t2.micro",
		},
	}
	poolfile := config.PoolFile{
		Version:   "1",
		Instances: []config.Instance{instance},
	}

	return &poolfile
}
