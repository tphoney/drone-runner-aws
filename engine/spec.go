// Copyright 2020 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package engine

import (
	"github.com/drone-runners/drone-runner-aws/types"
	"github.com/drone/runner-go/environ"
	"github.com/drone/runner-go/pipeline/runtime"

	lespec "github.com/harness/lite-engine/engine/spec"
)

type (
	// Spec provides the pipeline spec. This provides the
	// required instructions for reproducible pipeline
	// execution.
	Spec struct {
		Name                string            `json:"name,omitempty"`
		CloudInstance       CloudInstance     `json:"cloud_instance"`
		Files               []*lespec.File    `json:"files,omitempty"`
		Platform            types.Platform    `json:"platform"`
		Steps               []*Step           `json:"steps,omitempty"`
		Volumes             []*lespec.Volume  `json:"volumes,omitempty"`
		Network             lespec.Network    `json:"network"`
		OutputVars          map[string]string `json:"output_vars,omitempty"`
		OutputVariablesFile string            `json:"output_variables_file,omitempty"`
	}

	// CloudInstance provides basic instance information
	CloudInstance struct {
		PoolName string `json:"pool_name"`
		ID       string `json:"id,omitempty"`
		IP       string `json:"ip,omitempty"`
	}

	Step struct {
		lespec.Step
		DependsOn []string          `json:"depends_on,omitempty"`
		ErrPolicy runtime.ErrPolicy `json:"err_policy,omitempty"`
		RunPolicy runtime.RunPolicy `json:"run_policy,omitempty"`
	}
	// Secret represents a secret variable.
	// TODO: This type implements runtime.Secret unlike the one in LiteEngine. Move the interface methods to LE and remove the type.
	Secret lespec.Secret
)

//
// implements the Spec interface
//

func (s *Spec) StepLen() int              { return len(s.Steps) }
func (s *Spec) StepAt(i int) runtime.Step { return s.Steps[i] }
func (s *Spec) OutputVariablesToStep() map[string]string {
	// called by runner_go execer to return the output variables set in the previous steps.
	return s.OutputVars
}

//
// implements the Secret interface
//

func (s *Secret) GetName() string  { return s.Name }
func (s *Secret) GetValue() string { return string(s.Data) }
func (s *Secret) IsMasked() bool   { return s.Mask }

//
// implements the Step interface
//

func (s *Step) GetName() string                  { return s.Name }
func (s *Step) GetDependencies() []string        { return s.DependsOn }
func (s *Step) GetEnviron() map[string]string    { return s.Envs }
func (s *Step) SetEnviron(env map[string]string) { s.Envs = env }
func (s *Step) GetErrPolicy() runtime.ErrPolicy  { return s.ErrPolicy }
func (s *Step) GetRunPolicy() runtime.RunPolicy  { return s.RunPolicy }
func (s *Step) GetSecretAt(i int) runtime.Secret { return (*Secret)(s.Secrets[i]) }
func (s *Step) GetSecretLen() int                { return len(s.Secrets) }
func (s *Step) IsDetached() bool                 { return s.Detach }
func (s *Step) GetImage() string                 { return s.Image }
func (s *Step) Clone() runtime.Step {
	dst := new(Step)
	*dst = *s
	dst.Envs = environ.Combine(s.Envs)
	return dst
}
