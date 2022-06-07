package types

import (
	"database/sql/driver"
)

type InstanceState string
type ProviderType string

// Value converts the value to a sql string.
func (s InstanceState) Value() (driver.Value, error) {
	return string(s), nil
}

func (s ProviderType) Value() (driver.Value, error) {
	return string(s), nil
}

const (
	ProviderAmazon   = ProviderType("amazon")
	ProviderGoogle   = ProviderType("google")
	ProviderVMFusion = ProviderType("vmfusion")
	ProviderAnka     = ProviderType("anka")
)

// InstanceState type enumeration.
const (
	StateCreated = InstanceState("created")
	StateInUse   = InstanceState("inuse")
)

type Instance struct {
	ID           string        `db:"instance_id" json:"id"`
	Name         string        `db:"instance_name" json:"name"`
	Address      string        `db:"instance_address" json:"address"`
	Provider     ProviderType  `db:"instance_provider" json:"provider"`
	State        InstanceState `db:"instance_state" json:"state"`
	Pool         string        `db:"instance_pool" json:"pool"`
	Image        string        `db:"instance_image" json:"image"`
	Region       string        `db:"instance_region" json:"region"`
	Zone         string        `db:"instance_zone" json:"zone"`
	Size         string        `db:"instance_size" json:"size"`
	Platform     `json:"platform"`
	CAKey        []byte `db:"instance_ca_key" json:"ca_key"`
	CACert       []byte `db:"instance_ca_cert" json:"ca_cert"`
	TLSKey       []byte `db:"instance_tls_key" json:"tls_key"`
	TLSCert      []byte `db:"instance_tls_cert" json:"tls_cert"`
	Stage        string `db:"instance_stage" json:"stage"`
	Updated      int64  `db:"instance_updated" json:"updated"`
	Started      int64  `db:"instance_started" json:"started"`
	IsHibernated bool   `db:"is_hibernated" json:"is_hibernated"`
}

type InstanceCreateOpts struct {
	CAKey          []byte
	CACert         []byte
	TLSKey         []byte
	TLSCert        []byte
	LiteEnginePath string
	Platform
	PoolName   string
	RunnerName string
	Limit      int
	Pool       int
}

// Platform defines the target platform.
type Platform struct {
	OS      string `json:"os,omitempty" db:"instance_os"`
	Arch    string `json:"arch,omitempty" db:"instance_arch"`
	Variant string `json:"variant,omitempty" yaml:"variant,omitempty" db:"instance_variant"`
	Version string `json:"version,omitempty" yaml:"version,omitempty" db:"instance_version"`
	OSName  string `json:"os_name,omitempty" yaml:"os_name,omitempty" db:"instance_os_name"`
}

type QueryParams struct {
	Status InstanceState
	Stage  string
}

type StageOwner struct {
	StageID  string `db:"stage_id" json:"stage_id"`
	PoolName string `db:"pool_name" json:"pool_name"`
}
