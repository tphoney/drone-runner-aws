package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/drone-runners/drone-runner-aws/types"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type (
	PoolFile struct {
		Version   string     `json:"version" yaml:"version"`
		Instances []Instance `json:"instances" yaml:"instances"`
	}

	Instance struct {
		Name     string         `json:"name"`
		Default  bool           `json:"default"`
		Type     string         `json:"type"`
		Pool     int            `json:"pool"`
		Limit    int            `json:"limit"`
		Platform types.Platform `json:"platform,omitempty" yaml:"platform,omitempty"`
		Spec     interface{}    `json:"spec,omitempty"`
	}

	// Google specifies the configuration for a GCP instance.
	Google struct {
		Account      GoogleAccount     `json:"account,omitempty"  yaml:"account"`
		Image        string            `json:"image,omitempty" yaml:"image,omitempty"`
		Name         string            `json:"name,omitempty" yaml:"name,omitempty"`
		Tags         []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
		Size         string            `json:"size,omitempty" yaml:"size,omitempty"`
		MachineType  string            `json:"machine_type,omitempty" yaml:"machine_type"`
		UserData     string            `json:"user_data,omitempty" yaml:"user_data,omitempty"`
		UserDataPath string            `json:"user_data_path,omitempty" yaml:"user_data_path,omitempty"`
		UserDataKey  string            `json:"user_data_key,omitempty" yaml:"user_data_key,omitempty"`
		Disk         disk              `json:"disk,omitempty"`
		Network      string            `json:"network,omitempty" yaml:"network,omitempty"`
		Subnetwork   string            `json:"Subnetwork,omitempty" yaml:"Subnetwork,omitempty"`
		PrivateIP    bool              `json:"private_ip,omitempty"`
		Zone         []string          `json:"zone,omitempty" yaml:"zone"`
		Labels       map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
		Scopes       []string          `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	}

	GoogleAccount struct {
		ProjectID           string   `json:"project_id,omitempty"  yaml:"project_id"`
		JSONPath            string   `json:"json_path,omitempty"  yaml:"json_path"`
		Scopes              []string `json:"scopes,omitempty"  yaml:"scopes,omitempty"`
		ServiceAccountEmail string   `json:"service_account_email,omitempty"  yaml:"service_account_email,omitempty"`
	}

	// Amazon specifies the configuration for an AWS instance.
	Amazon struct {
		Account       AmazonAccount     `json:"account,omitempty"`
		Name          string            `json:"name,omitempty" yaml:"name,omitempty"`
		Size          string            `json:"size,omitempty"`
		SizeAlt       string            `json:"size_alt,omitempty" yaml:"size_alt,omitempty"`
		AMI           string            `json:"ami,omitempty"`
		VPC           string            `json:"vpc,omitempty" yaml:"vpc,omitempty"`
		Tags          map[string]string `json:"tags,omitempty" yaml:"tags,omitempty"`
		Type          string            `json:"type,omitempty" yaml:"type,omitempty"`
		UserData      string            `json:"user_data,omitempty" yaml:"user_data,omitempty"`
		UserDataPath  string            `json:"user_data_Path,omitempty" yaml:"user_data_Path,omitempty"`
		Disk          disk              `json:"disk,omitempty" yaml:"disk,omitempty"`
		Network       AmazonNetwork     `json:"network,omitempty" yaml:"network,omitempty"`
		DeviceName    string            `json:"device_name,omitempty" yaml:"device_name,omitempty"`
		IamProfileArn string            `json:"iam_profile_arn,omitempty" yaml:"iam_profile_arn,omitempty"`
		MarketType    string            `json:"market_type,omitempty" yaml:"market_type,omitempty"`
		RootDirectory string            `json:"root_directory,omitempty" yaml:"root_directory,omitempty"`
		Hibernate     bool              `json:"hibernate,omitempty"`
		User          string            `json:"user,omitempty" yaml:"user,omitempty"`
	}
	AmazonAccount struct {
		AccessKeyID      string `json:"access_key_id,omitempty"  yaml:"access_key_id"`
		AccessKeySecret  string `json:"access_key_secret,omitempty" yaml:"access_key_secret"`
		Region           string `json:"region,omitempty"`
		Retries          int    `json:"retries,omitempty" yaml:"retries,omitempty"`
		AvailabilityZone string `json:"availability_zone,omitempty" yaml:"availability_zone,omitempty"`
		KeyPairName      string `json:"key_pair_name,omitempty" yaml:"key_pair_name,omitempty"`
	}
	// AmazonNetwork provides AmazonNetwork settings.
	AmazonNetwork struct {
		VPC               string   `json:"vpc,omitempty"`
		VPCSecurityGroups []string `json:"vpc_security_group_ids,omitempty" yaml:"vpc_security_groups"`
		SecurityGroups    []string `json:"security_groups,omitempty" yaml:"security_groups"`
		SubnetID          string   `json:"subnet_id,omitempty" yaml:"subnet_id"`
		PrivateIP         bool     `json:"private_ip,omitempty" yaml:"private_ip"`
	}

	// VMFusion specifies the configuration for a VMware instance.
	VMFusion struct {
		Account struct {
			Username string `json:"username,omitempty"  yaml:"username"`
			Password string `json:"password,omitempty"  yaml:"password"`
		}
		ISO           string `json:"iso,omitempty"`
		Name          string `json:"name,omitempty" yaml:"name"`
		Memory        int64  `json:"memory,omitempty" yaml:"memory"`
		CPU           int64  `json:"cpu,omitempty" yaml:"cpu"`
		VDiskPath     string `json:"v_disk_path,omitempty" yaml:"v_disk_path"`
		UserData      string `json:"user_data,omitempty"`
		UserDataPath  string `json:"user_data_Path,omitempty" yaml:"user_data_Path,omitempty"`
		StorePath     string `json:"store_path,omitempty" yaml:"store_path"`
		RootDirectory string `json:"root_directory,omitempty" yaml:"root_directory"`
	}
	// Anka specifies the configuration for an Anka instance.
	Anka struct {
		Account struct {
			Username string `json:"username,omitempty"  yaml:"username"`
			Password string `json:"password,omitempty"  yaml:"password"`
		}
		VMID          string `json:"vm_id,omitempty" yaml:"vm_id"`
		RootDirectory string `json:"root_directory,omitempty" yaml:"root_directory"`
		UserData      string `json:"user_data,omitempty" yaml:"user_data"`
		UserDataPath  string `json:"user_data_Path,omitempty" yaml:"user_data_Path,omitempty"`
	}

	// disk provides disk size and type.
	disk struct {
		Size int64  `json:"size,omitempty"`
		Type string `json:"type,omitempty"`
		Iops int64  `json:"iops,omitempty"`
	}
)

type EnvConfig struct {
	Debug bool `envconfig:"DRONE_DEBUG"`
	Trace bool `envconfig:"DRONE_TRACE"`

	Client struct {
		Address    string `ignored:"true"`
		Proto      string `envconfig:"DRONE_RPC_PROTO" default:"http"`
		Host       string `envconfig:"DRONE_RPC_HOST"`
		Secret     string `envconfig:"DRONE_RPC_SECRET"`
		SkipVerify bool   `envconfig:"DRONE_RPC_SKIP_VERIFY"`
		Dump       bool   `envconfig:"DRONE_RPC_DUMP_HTTP"`
		DumpBody   bool   `envconfig:"DRONE_RPC_DUMP_HTTP_BODY"`
	}

	Dashboard struct {
		Disabled bool   `envconfig:"DRONE_UI_DISABLE"`
		Username string `envconfig:"DRONE_UI_USERNAME"`
		Password string `envconfig:"DRONE_UI_PASSWORD"`
		Realm    string `envconfig:"DRONE_UI_REALM" default:"MyRealm"`
	}

	Server struct {
		Port  string `envconfig:"DRONE_HTTP_BIND" default:":3000"`
		Proto string `envconfig:"DRONE_HTTP_PROTO"`
		Host  string `envconfig:"DRONE_HTTP_HOST"`
		Acme  bool   `envconfig:"DRONE_HTTP_ACME"`
	}

	Runner struct {
		Name        string            `envconfig:"DRONE_RUNNER_NAME"`
		Capacity    int               `envconfig:"DRONE_RUNNER_CAPACITY" default:"6"`
		Procs       int64             `envconfig:"DRONE_RUNNER_MAX_PROCS"`
		Environ     map[string]string `envconfig:"DRONE_RUNNER_ENVIRON"`
		EnvFile     string            `envconfig:"DRONE_RUNNER_ENV_FILE"`
		Secrets     map[string]string `envconfig:"DRONE_RUNNER_SECRETS"`
		Labels      map[string]string `envconfig:"DRONE_RUNNER_LABELS"`
		NetworkOpts map[string]string `envconfig:"DRONE_RUNNER_NETWORK_OPTS"`
		Volumes     []string          `envconfig:"DRONE_RUNNER_VOLUMES"`
	}

	AWS struct {
		AccessKeyID     string `envconfig:"AWS_ACCESS_KEY_ID"`
		AccessKeySecret string `envconfig:"AWS_ACCESS_KEY_SECRET"`
		Region          string `envconfig:"AWS_DEFAULT_REGION" default:"us-east-2"`
	}

	Google struct {
		ProjectID string `envconfig:"GOOGLE_PROJECT_ID"`
		JSONPath  string `envconfig:"GOOGLE_JSON_PATH" default:"~/.config/gcloud/application_default_credentials.json"`
		Zone      string `envconfig:"GOOGLE_ZONE" default:"northamerica-northeast1-a"`
	}

	Limit struct {
		Repos   []string `envconfig:"DRONE_LIMIT_REPOS"`
		Events  []string `envconfig:"DRONE_LIMIT_EVENTS"`
		Trusted bool     `envconfig:"DRONE_LIMIT_TRUSTED"`
	}

	Settings struct {
		LiteEnginePath string `envconfig:"DRONE_LITE_ENGINE_PATH" default:"https://github.com/harness/lite-engine/releases/download/v0.1.0/"`
		ReusePool      bool   `envconfig:"DRONE_REUSE_POOL" default:"false"`
		BusyMaxAge     int64  `envconfig:"DRONE_SETTINGS_BUSY_MAX_AGE" default:"24"`
		FreeMaxAge     int64  `envconfig:"DRONE_SETTINGS_FREE_MAX_AGE" default:"720"`
		MinPoolSize    int    `envconfig:"DRONE_MIN_POOL_SIZE" default:"1"`
		MaxPoolSize    int    `envconfig:"DRONE_MAX_POOL_SIZE" default:"2"`
	}

	Environ struct {
		Endpoint   string `envconfig:"DRONE_ENV_PLUGIN_ENDPOINT"`
		Token      string `envconfig:"DRONE_ENV_PLUGIN_TOKEN"`
		SkipVerify bool   `envconfig:"DRONE_ENV_PLUGIN_SKIP_VERIFY"`
	}

	Secret struct {
		Endpoint   string `envconfig:"DRONE_SECRET_PLUGIN_ENDPOINT"`
		Token      string `envconfig:"DRONE_SECRET_PLUGIN_TOKEN"`
		SkipVerify bool   `envconfig:"DRONE_SECRET_PLUGIN_SKIP_VERIFY"`
	}

	Docker struct {
		Config string `envconfig:"DRONE_DOCKER_CONFIG"`
		Stream bool   `envconfig:"DRONE_DOCKER_STREAM_PULL" default:"true"` // TODO: Currently unused
	}

	Registry struct {
		Endpoint   string `envconfig:"DRONE_REGISTRY_PLUGIN_ENDPOINT"`
		Token      string `envconfig:"DRONE_REGISTRY_PLUGIN_TOKEN"`
		SkipVerify bool   `envconfig:"DRONE_REGISTRY_PLUGIN_SKIP_VERIFY"`
	}

	Database struct {
		Driver     string `envconfig:"DRONE_DATABASE_DRIVER" default:"sqlite3"`
		Datasource string `envconfig:"DRONE_DATABASE_DATASOURCE" default:"database.sqlite3"`
	}
}

// legacy environment variables. the key is the legacy
// variable name, and the value is the new variable name.
var legacy = map[string]string{
	// "DRONE_VARIABLE_OLD": "DRONE_VARIABLE_NEW"
}

func FromEnviron() (EnvConfig, error) {
	// loop through legacy environment variable and, if set
	// rewrite to the new variable name.
	for k, v := range legacy {
		if s, ok := os.LookupEnv(k); ok {
			os.Setenv(v, s)
		}
	}

	var config EnvConfig
	err := envconfig.Process("", &config)
	if err != nil {
		return config, err
	}
	if config.Runner.Environ == nil {
		config.Runner.Environ = map[string]string{}
	}
	if config.Runner.Name == "" {
		config.Runner.Name, _ = os.Hostname()
	}
	if config.Dashboard.Password == "" {
		config.Dashboard.Disabled = true
	}
	config.Client.Address = fmt.Sprintf(
		"%s://%s",
		config.Client.Proto,
		config.Client.Host,
	)

	// environment variables can be sourced from a separate
	// file. These variables are loaded and appended to the
	// environment list.
	if file := config.Runner.EnvFile; file != "" {
		envs, err := godotenv.Read(file)
		if err != nil {
			return config, err
		}
		for k, v := range envs {
			config.Runner.Environ[k] = v
		}
	}

	return config, nil
}

// UnmarshalJSON implement the json.Unmarshaler interface.
func (s *Instance) UnmarshalJSON(data []byte) error {
	type S Instance
	type T struct {
		*S
		Spec json.RawMessage `json:"spec"`
	}
	obj := &T{S: (*S)(s)}
	if err := json.Unmarshal(data, obj); err != nil {
		return err
	}
	switch s.Type {
	case string(types.ProviderAmazon), "aws":
		s.Spec = new(Amazon)
	case string(types.ProviderGoogle), "gcp":
		s.Spec = new(Google)
	case string(types.ProviderVMFusion):
		s.Spec = new(VMFusion)
	case string(types.ProviderAnka):
		s.Spec = new(Anka)
	default:
		return fmt.Errorf("unknown instance type %s", s.Type)
	}
	return json.Unmarshal(obj.Spec, s.Spec)
}
