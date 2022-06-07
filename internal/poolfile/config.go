package poolfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

const (
	DefaultPoolName = "testpool"
)

func ProcessPool(poolFile *config.PoolFile, runnerName string) ([]drivers.Pool, error) {
	var pools = []drivers.Pool{}

	for i := range poolFile.Instances {
		instance := poolFile.Instances[i]
		switch instance.Type {
		case string(types.ProviderVMFusion):
			var v, ok = instance.Spec.(*config.VMFusion)
			if !ok {
				logrus.Errorln("unable to parse pool file")
			}
			// set platform defaults
			platform, platformErr := vmfusion.SetPlatformDefaults(&instance.Platform)
			if platformErr != nil {
				logrus.WithError(platformErr).WithField("provider", instance.Type)
			}
			instance.Platform = *platform

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
				logrus.WithError(err).WithField("provider", instance.Type)
			}
			pool := mapPool(&instance, runnerName)

			pool.Driver = driver
			pools = append(pools, pool)
		case string(types.ProviderAmazon):
			var a, ok = instance.Spec.(*config.Amazon)
			if !ok {
				logrus.Errorln("unable to parse pool file")
			}
			// set platform defaults
			platform, platformErr := amazon.SetPlatformDefaults(&instance.Platform)
			if platformErr != nil {
				logrus.WithError(platformErr).WithField("provider", instance.Type)
			}
			instance.Platform = *platform
			var driver, err = amazon.New(
				amazon.WithAccessKeyID(a.Account.AccessKeyID),
				amazon.WithSecretAccessKey(a.Account.AccessKeySecret),
				amazon.WithZone(a.Account.AvailabilityZone),
				amazon.WithKeyPair(a.Account.KeyPairName),
				amazon.WithDeviceName(a.DeviceName, instance.Platform.OSName),
				amazon.WithRootDirectory(a.RootDirectory),
				amazon.WithAMI(a.AMI),
				amazon.WithVpc(a.VPC),
				amazon.WithUser(a.User, instance.Platform.OS),
				amazon.WithRegion(a.Account.Region, a.Account.Region),
				amazon.WithRetries(a.Account.Retries),
				amazon.WithPrivateIP(a.Network.PrivateIP),
				amazon.WithSecurityGroup(a.Network.SecurityGroups...),
				amazon.WithSize(a.Size, instance.Platform.Arch),
				amazon.WithSizeAlt(a.SizeAlt),
				amazon.WithSubnet(a.Network.SubnetID),
				amazon.WithUserData(a.UserData, a.UserDataPath),
				amazon.WithVolumeSize(a.Disk.Size),
				amazon.WithVolumeType(a.Disk.Type),
				amazon.WithVolumeIops(a.Disk.Iops, a.Disk.Type),
				amazon.WithIamProfileArn(a.IamProfileArn),
				amazon.WithMarketType(a.MarketType),
				amazon.WithTags(a.Tags),
				amazon.WithHibernate(a.Hibernate),
			)
			if err != nil {
				logrus.WithError(err).WithField("provider", instance.Type)
			}
			pool := mapPool(&instance, runnerName)
			pool.Driver = driver
			pools = append(pools, pool)
		case string(types.ProviderGoogle):
			var g, ok = instance.Spec.(*config.Google)
			if !ok {
				logrus.Errorln("unable to parse pool file")
			}
			// set platform defaults
			platform, platformErr := anka.SetPlatformDefaults(&instance.Platform)
			if platformErr != nil {
				logrus.WithError(platformErr).WithField("provider", instance.Type)
			}
			instance.Platform = *platform
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
				google.WithUserDataKey(g.UserDataKey, instance.Platform.OS),
			)
			if err != nil {
				logrus.WithError(err).WithField("provider", instance.Type)
			}
			pool := mapPool(&instance, runnerName)
			pool.Driver = driver
			pools = append(pools, pool)
		case string(types.ProviderAnka):
			var ak, ok = instance.Spec.(*config.Anka)
			if !ok {
				logrus.Errorln("unable to parse pool file")
			}
			// set platform defaults
			platform, platformErr := anka.SetPlatformDefaults(&instance.Platform)
			if platformErr != nil {
				logrus.WithError(platformErr).WithField("provider", instance.Type)
			}
			instance.Platform = *platform
			driver, err := anka.New(
				anka.WithUsername(ak.Account.Username),
				anka.WithPassword(ak.Account.Password),
				anka.WithRootDirectory(ak.RootDirectory),
				anka.WithUserData(ak.UserData, ak.UserDataPath),
				anka.WithVMID(ak.VMID),
			)
			if err != nil {
				logrus.WithError(err).WithField("provider", instance.Type)
			}
			pool := mapPool(&instance, runnerName)
			pool.Driver = driver
			pools = append(pools, pool)
		default:
			return nil, fmt.Errorf("unknown instance type %s", instance.Type)
		}
	}
	return pools, nil
}

func mapPool(instance *config.Instance, runnerName string) (pool drivers.Pool) {
	// set pool defaults
	if instance.Pool < 0 {
		instance.Pool = 0
	}
	if instance.Limit <= 0 {
		instance.Limit = 100
	}
	if instance.Pool > instance.Limit {
		instance.Limit = instance.Pool
	}

	pool = drivers.Pool{
		RunnerName: runnerName,
		Name:       instance.Name,
		MaxSize:    instance.Limit,
		MinSize:    instance.Pool,
		Platform:   instance.Platform,
	}
	return pool
}

func ConfigPoolFile(path, providerType string, conf *config.EnvConfig) (pool *config.PoolFile, err error) {
	if path == "" {
		logrus.Infof("no pool file provided, creating in memory pool for %s", providerType)
		// generate a pool file
		switch providerType {
		case string(types.ProviderAmazon):
			// do we have the creds?
			if conf.AWS.AccessKeyID == "" || conf.AWS.AccessKeySecret == "" {
				return pool, fmt.Errorf("%s:missing credentials in env variables 'AWS_ACCESS_KEY_ID' and 'AWS_ACCESS_KEY_SECRET'", providerType)
			}
			return createAmazonPool(conf.AWS.AccessKeyID, conf.AWS.AccessKeySecret, conf.AWS.Region, conf.Settings.MinPoolSize, conf.Settings.MaxPoolSize), nil
		case string(types.ProviderGoogle):
			// do we have the creds?
			if conf.Google.ProjectID == "" {
				return pool, fmt.Errorf("%s:missing credentials in env variables 'GOOGLE_PROJECT_ID'", providerType)
			}
			absPath := filepath.Clean(conf.Google.JSONPath)
			if strings.HasPrefix(absPath, "~/") {
				dirname, _ := os.UserHomeDir()
				absPath = filepath.Join(dirname, absPath[2:])
			}
			_, pathErr := os.Stat(absPath)
			if errors.Is(pathErr, os.ErrNotExist) {
				return pool, fmt.Errorf("%s:unable to find credentials file at '%s'", providerType, conf.Google.JSONPath)
			}
			return createGooglePool(conf.Google.ProjectID, conf.Google.JSONPath, conf.Google.Zone, conf.Settings.MinPoolSize, conf.Settings.MaxPoolSize), nil
		default:
			err = fmt.Errorf("unknown provider type %s, unable to create pool file in memory", providerType)
			return pool, err
		}
	}
	pool, err = config.ParseFile(path)
	if err != nil {
		logrus.WithError(err).
			WithField("path", path).
			Errorln("exec: unable to parse pool file")
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

func createAmazonPool(accessKeyID, accessKeySecret, region string, minPoolSize, maxPoolSize int) *config.PoolFile {
	instance := config.Instance{
		Name:    DefaultPoolName,
		Default: true,
		Type:    string(types.ProviderAmazon),
		Pool:    minPoolSize,
		Limit:   maxPoolSize,
		Platform: types.Platform{
			Arch: oshelp.ArchAMD64,
			OS:   oshelp.OSLinux,
		},
		Spec: &config.Amazon{
			Account: config.AmazonAccount{
				Region:          region,
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

func createGooglePool(projectID, path, zone string, minPoolSize, maxPoolSize int) *config.PoolFile {
	instance := config.Instance{
		Name:    DefaultPoolName,
		Default: true,
		Type:    string(types.ProviderGoogle),
		Pool:    minPoolSize,
		Limit:   maxPoolSize,
		Platform: types.Platform{
			Arch: "amd64",
			OS:   "linux",
		},
		Spec: &config.Google{
			Account: config.GoogleAccount{
				ProjectID: projectID,
				JSONPath:  path,
			},
			Image:       "projects/ubuntu-os-pro-cloud/global/images/ubuntu-pro-1804-bionic-v20220131",
			MachineType: "e2-small",
			Zone:        []string{zone},
		},
	}
	poolfile := config.PoolFile{
		Version:   "1",
		Instances: []config.Instance{instance},
	}

	return &poolfile
}
