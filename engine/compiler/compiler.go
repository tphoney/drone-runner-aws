// Copyright 2020 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package compiler

import (
	"context"
	"fmt"

	"github.com/drone/runner-go/labels"

	"github.com/drone-runners/drone-runner-aws/engine"
	"github.com/drone-runners/drone-runner-aws/engine/resource"
	"github.com/drone-runners/drone-runner-aws/internal/vmpool"
	"github.com/drone-runners/drone-runner-aws/oshelp"

	"github.com/drone/runner-go/clone"
	"github.com/drone/runner-go/environ"
	"github.com/drone/runner-go/environ/provider"
	"github.com/drone/runner-go/manifest"
	"github.com/drone/runner-go/pipeline/runtime"
	"github.com/drone/runner-go/secret"
	lespec "github.com/harness/lite-engine/engine/spec"

	"github.com/dchest/uniuri"
)

// random generator function
var random = func() string {
	return "drone-" + uniuri.NewLen(20) //nolint:gomnd
}

// Compiler compiles the Yaml configuration file to an
// intermediate representation optimized for simple execution.
type Compiler struct {
	// Environ provides a set of environment variables that should be added to each pipeline step by default.
	Environ provider.Provider
	// Secret returns a named secret value that can be injected into the pipeline step.
	Secret secret.Provider
	// Pools is a map of named pools that can be referenced by a pipeline.
	PoolManager *vmpool.Manager
}

// Compile compiles the configuration file.
func (c *Compiler) Compile(ctx context.Context, args runtime.CompilerArgs) runtime.Spec { //nolint:gocritic
	pipeline := args.Pipeline.(*resource.Pipeline)
	spec := &engine.Spec{}

	spec.Name = pipeline.Name

	// create system labels
	systemLabels := labels.Combine(
		labels.FromRepo(args.Repo),
		labels.FromBuild(args.Build),
		labels.FromStage(args.Stage),
		labels.FromSystem(args.System),
		labels.WithTimeout(args.Repo),
	)
	targetPool := pipeline.Pool.Use
	pool := c.PoolManager.Get(targetPool)

	pipelineOS := pool.GetOS()
	pipelineRoot := pool.GetRootDir()

	// move the pool from the `mapping of pools` into the spec of this pipeline.
	spec.CloudInstance.PoolName = targetPool

	// creates a home directory in the root.
	// note: mkdirall fails on windows so we need to create all directories in the tree.
	homedir := oshelp.JoinPaths(pipelineOS, pipelineRoot, "home", "drone")
	spec.Files = append(spec.Files,
		&lespec.File{
			Path:  oshelp.JoinPaths(pipelineOS, pipelineRoot, "home"),
			Mode:  0700,
			IsDir: true,
		}, &lespec.File{
			Path:  homedir,
			Mode:  0700,
			IsDir: true,
		})

	// creates a source directory in the root.
	// note: mkdirall fails on windows so we need to create all
	// directories in the tree.
	sourcedir := oshelp.JoinPaths(pipelineOS, pipelineRoot, "drone", "src")
	spec.Files = append(spec.Files,
		&lespec.File{
			Path:  oshelp.JoinPaths(pipelineOS, pipelineRoot, "drone"),
			Mode:  0700,
			IsDir: true,
		},
		&lespec.File{
			Path:  sourcedir,
			Mode:  0700,
			IsDir: true,
		},
		&lespec.File{
			Path:  oshelp.JoinPaths(pipelineOS, pipelineRoot, "opt"),
			Mode:  0700,
			IsDir: true,
		})

	// creates the netrc file
	if args.Netrc != nil && args.Netrc.Password != "" {
		netrcfile := oshelp.GetNetrc(pipelineOS)
		netrcpath := oshelp.JoinPaths(pipelineOS, homedir, netrcfile)
		netrcdata := fmt.Sprintf(
			"machine %s login %s password %s",
			args.Netrc.Machine,
			args.Netrc.Login,
			args.Netrc.Password,
		)
		spec.Files = append(spec.Files,
			&lespec.File{
				Path: netrcpath,
				Mode: 0600,
				Data: netrcdata,
			})
	}

	// list the global environment variables
	globals, _ := c.Environ.List(ctx, &provider.Request{
		Build: args.Build,
		Repo:  args.Repo,
	})

	// create the default environment variables.
	envs := environ.Combine(
		provider.ToMap(
			provider.FilterUnmasked(globals),
		),
		args.Build.Params,
		pipeline.Environment,
		environ.Proxy(),
		environ.System(args.System),
		environ.Repo(args.Repo),
		environ.Build(args.Build),
		environ.Stage(args.Stage),
		environ.Link(args.Repo, args.Build, args.System),
		clone.Environ(clone.Config{
			SkipVerify: pipeline.Clone.SkipVerify,
			Trace:      pipeline.Clone.Trace,
			User: clone.User{
				Name:  args.Build.AuthorName,
				Email: args.Build.AuthorEmail,
			},
		}),
		map[string]string{
			"HOME":                homedir,
			"HOMEPATH":            homedir, // for windows
			"USERPROFILE":         homedir, // for windows
			"DRONE_HOME":          sourcedir,
			"DRONE_WORKSPACE":     sourcedir,
			"GIT_TERMINAL_PROMPT": "0",
		},
	)

	match := manifest.Match{
		Action:   args.Build.Action,
		Cron:     args.Build.Cron,
		Ref:      args.Build.Ref,
		Repo:     args.Repo.Slug,
		Instance: args.System.Host,
		Target:   args.Build.Deploy,
		Event:    args.Build.Event,
		Branch:   args.Build.Target,
	}

	// create the clone step, maybe
	if !pipeline.Clone.Disable {
		clonepath := oshelp.JoinPaths(pipelineOS, pipelineRoot, "opt", oshelp.GetExt(pipelineOS, "clone"))
		clonefile := oshelp.GenScript(pipelineOS,
			clone.Commands(
				clone.Args{
					Branch: args.Build.Target,
					Commit: args.Build.After,
					Ref:    args.Build.Ref,
					Remote: args.Repo.HTTPURL,
				},
			),
		)
		var cloneEntrypoint []string
		if pipelineOS == "windows" {
			cloneEntrypoint = []string{"powershell"}
		} else {
			cloneEntrypoint = []string{"sh", "-c"}
		}
		spec.Steps = append(spec.Steps, &engine.Step{
			Step: lespec.Step{
				ID:         random(),
				Name:       "clone",
				Entrypoint: cloneEntrypoint,
				Command:    []string{clonefile},
				Envs:       envs,
				Secrets:    []*lespec.Secret{},
				WorkingDir: sourcedir,
				Files: []*lespec.File{
					{
						Path: clonepath,
						Mode: 0700,
						Data: clonefile,
					},
				},
			},
			DependsOn: nil,
			ErrPolicy: runtime.ErrFail,
			RunPolicy: runtime.RunAlways,
		})
	}

	// create volumes map, name of volume and real life path
	for _, v := range pipeline.Volumes {
		id := random()
		path := ""

		src := new(lespec.Volume)
		if v.EmptyDir != nil {
			path = oshelp.JoinPaths(pipelineOS, pipelineRoot, id)
			src.EmptyDir = &lespec.VolumeEmptyDir{
				ID:     id,
				Name:   v.Name,
				Labels: systemLabels,
			}
		} else if v.HostPath != nil {
			path = v.HostPath.Path
			src.HostPath = &lespec.VolumeHostPath{
				ID:     id,
				Name:   v.Name,
				Path:   path,
				Labels: systemLabels,
			}
		} else {
			continue
		}

		spec.Volumes = append(spec.Volumes, src)
	}

	// services are the same as steps, but are executed first and are detached.
	for _, src := range pipeline.Services {
		src.Detach = true
	}
	// combine steps + services
	combinedSteps := append(pipeline.Services, pipeline.Steps...) //nolint:gocritic // creating a new slice is ok
	// create steps
	for _, src := range combinedSteps {
		stepEnv := environ.Combine(envs, environ.Expand(convertStaticEnv(src.Environment)))

		var files []*lespec.File
		var volumes []*lespec.VolumeMount
		var command []string
		var entrypoint []string
		stepID := random()

		if len(src.Commands) > 0 {
			// build the script of commands we will execute
			scriptToExecute := oshelp.GenScript(pipelineOS, src.Commands)
			scriptPath := oshelp.JoinPaths(pipelineOS, pipelineRoot, "opt", oshelp.GetExt(pipelineOS, stepID))
			files = append(files, &lespec.File{
				Path: scriptPath,
				Mode: 0700,
				Data: scriptToExecute,
			})
			command = append(command, scriptPath)
		}

		// set entrypoint if running on the host or if the container has commands
		if src.Image == "" || (src.Image != "" && len(src.Commands) > 0) {
			if pipelineOS == "windows" {
				entrypoint = []string{"powershell"}
			} else {
				entrypoint = []string{"sh", "-c"}
			}
		}

		dst := &engine.Step{
			Step: lespec.Step{
				ID:         stepID,
				Name:       src.Name,
				Command:    command,
				Detach:     src.Detach,
				Envs:       stepEnv,
				Entrypoint: entrypoint,
				Files:      files,
				Image:      src.Image,
				Secrets:    convertSecretEnv(src.Environment),
				WorkingDir: sourcedir,
				Volumes:    volumes,
			},
			DependsOn: src.DependsOn,
			RunPolicy: runtime.RunOnSuccess,
		}
		spec.Steps = append(spec.Steps, dst)

		// set the pipeline step run policy. steps run on success by default, but may be optionally configured to run on failure.
		if isRunAlways(src) {
			dst.RunPolicy = runtime.RunAlways
		} else if isRunOnFailure(src) {
			dst.RunPolicy = runtime.RunOnFailure
		}

		// if the pipeline step has unmet conditions the step is automatically skipped.
		if !src.When.Match(match) {
			dst.RunPolicy = runtime.RunNever
		}
	}

	if !isGraph(spec) {
		configureSerial(spec)
	} else if !pipeline.Clone.Disable {
		configureCloneDeps(spec)
	} else if pipeline.Clone.Disable {
		removeCloneDeps(spec)
	}

	for _, step := range spec.Steps {
		for _, s := range step.Secrets {
			actualSecret, ok := c.findSecret(ctx, args, s.Name)
			if ok {
				s.Data = []byte(actualSecret)
			}
		}
	}

	return spec
}

// helper function attempts to find and return the named secret. from the secret provider.
func (c *Compiler) findSecret(ctx context.Context, args runtime.CompilerArgs, name string) (s string, ok bool) { //nolint:gocritic // its complex but standard
	if name == "" {
		return
	}
	// source secrets from the global secret provider and the repository secret provider.
	p := secret.Combine(
		args.Secret,
		c.Secret,
	)
	// fine the secret from the provider. please note we
	// currently ignore errors if the secret is not found,
	// which is something that we'll need to address in the
	// next major (breaking) release.
	found, _ := p.Find(ctx, &secret.Request{
		Name:  name,
		Build: args.Build,
		Repo:  args.Repo,
		Conf:  args.Manifest,
	})
	if found == nil {
		return
	}
	return found.Data, true
}
