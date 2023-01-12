// Copyright 2020 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package compiler

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/drone-runners/drone-runner-aws/engine"
	"github.com/drone-runners/drone-runner-aws/engine/resource"
	"github.com/drone-runners/drone-runner-aws/internal/drivers"
	"github.com/drone-runners/drone-runner-aws/internal/encoder"
	"github.com/drone-runners/drone-runner-aws/internal/oshelp"
	"github.com/drone/drone-go/drone"

	"github.com/drone/runner-go/clone"
	"github.com/drone/runner-go/environ"
	"github.com/drone/runner-go/environ/provider"
	"github.com/drone/runner-go/labels"
	"github.com/drone/runner-go/manifest"
	"github.com/drone/runner-go/pipeline/runtime"
	"github.com/drone/runner-go/registry"
	"github.com/drone/runner-go/registry/auths"
	"github.com/drone/runner-go/secret"
	leimage "github.com/harness/lite-engine/engine/docker/image"
	lespec "github.com/harness/lite-engine/engine/spec"
)

type (
	// Tmate defines tmate settings.
	Tmate struct {
		Image   string
		Enabled bool
		Server  string
		Port    string
		RSA     string
		ED25519 string
	}

	// Compiler compiles the Yaml configuration file to an intermediate representation optimized for simple execution.
	Compiler struct {
		// Environ provides a set of environment variables that should be added to each pipeline step by default.
		Environ provider.Provider

		// NetworkOpts provides a set of network options that
		// are used when creating the docker network.
		NetworkOpts map[string]string

		// Secret returns a named secret value that can be injected into the pipeline step.
		Secret secret.Provider

		// Pools is a map of named pools that can be referenced by a pipeline.
		PoolManager *drivers.Manager

		// Registry returns a list of registry credentials that can be
		// used to pull private container images.
		Registry registry.Provider

		// Volumes provides a set of volumes that should be mounted to each pipeline container
		Volumes []string

		// Tmate provides global configration options for tmate live debugging.
		Tmate

		OutputVariablesFile string
	}
)

// Compile compiles the configuration file.
func (c *Compiler) Compile(ctx context.Context, args runtime.CompilerArgs) runtime.Spec { //nolint:gocritic,gocyclo,funlen
	pipeline := args.Pipeline.(*resource.Pipeline)
	spec := &engine.Spec{}

	spec.Platform = pipeline.Platform

	spec.Name = pipeline.Name

	// get OS and the root directory (where the work directory and everything else will be placed)
	targetPool := pipeline.Pool.Use

	if targetPool == "" {
		targetPool = c.PoolManager.MatchPoolNameFromPlatform(&pipeline.Platform)
	}

	pipelinePlatform, pipelineRoot := c.PoolManager.Inspect(targetPool)

	// move the pool from the `mapping of pools` into the spec of this pipeline.
	spec.CloudInstance.PoolName = targetPool

	// create directories
	// * homeDir is home directory on the host machine where netrc file will be placed
	// * sourceDir is directory on the host machine where source code will be pulled
	// * scriptDir is directory on the host machine where script files (with commands) will be placed
	directories, homeDir, sourceDir := createDirectories(pipelinePlatform.OS, pipelineRoot)
	spec.Files = append(spec.Files, directories...)

	// create netrc file if needed
	if netrc := args.Netrc; netrc != nil && netrc.Password != "" {
		netrcfile := oshelp.GetNetrc(pipelinePlatform.OS)
		netrcpath := oshelp.JoinPaths(pipelinePlatform.OS, homeDir, netrcfile)
		netrcdata := fmt.Sprintf(
			"machine %s login %s password %s",
			netrc.Machine,
			netrc.Login,
			netrc.Password,
		)

		spec.Files = append(spec.Files, &lespec.File{Path: netrcpath, Mode: 0600, Data: netrcdata})
	}

	// create the default environment variables.
	globals, _ := c.Environ.List(ctx, &provider.Request{
		Build: args.Build,
		Repo:  args.Repo,
	})
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
			"HOME":                homeDir,
			"HOMEPATH":            homeDir, // for windows
			"USERPROFILE":         homeDir, // for windows
			"DRONE_WORKSPACE":     sourceDir,
			"GIT_TERMINAL_PROMPT": "0",
		},
	)

	// outputvariables file
	envs["DRONE_ENV"] = path.Join(sourceDir, c.OutputVariablesFile)
	spec.OutputVariablesFile = envs["DRONE_ENV"]

	// add tmate settings to the environment
	if c.Tmate.Server != "" {
		envs["DRONE_TMATE_HOST"] = c.Tmate.Server
		envs["DRONE_TMATE_PORT"] = c.Tmate.Port
		envs["DRONE_TMATE_FINGERPRINT_RSA"] = c.Tmate.RSA
		envs["DRONE_TMATE_FINGERPRINT_ED25519"] = c.Tmate.ED25519
	}

	// create the clone step, maybe
	if !pipeline.Clone.Disable {
		cloneScript := oshelp.GenScript(pipelinePlatform.OS, pipelinePlatform.Arch,
			clone.Commands(
				clone.Args{
					Branch: args.Build.Target,
					Commit: args.Build.After,
					Ref:    args.Build.Ref,
					Remote: args.Repo.HTTPURL,
				},
			),
		)
		clonePath := oshelp.JoinPaths(pipelinePlatform.OS, pipelineRoot, "opt", oshelp.GetExt(pipelinePlatform.OS, "clone"))

		entrypoint := oshelp.GetEntrypoint(pipelinePlatform.OS)
		command := []string{clonePath}

		spec.Steps = append(spec.Steps, &engine.Step{
			Step: lespec.Step{
				ID:         oshelp.Random(),
				Name:       "clone",
				Entrypoint: entrypoint,
				Command:    command,
				Envs:       envs,
				Secrets:    []*lespec.Secret{},
				WorkingDir: sourceDir,
				Files: []*lespec.File{
					{
						Path: clonePath,
						Mode: 0700,
						Data: cloneScript,
					},
				},
			},
			DependsOn: nil,
			ErrPolicy: runtime.ErrFail,
			RunPolicy: runtime.RunAlways,
		})
	}
	// match object is used to determine is a step should be executed or not
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
	// create steps
	haveImageSteps := false // should be true if there is at least one step that uses an image
	for _, src := range pipeline.Services {
		src.Detach = true // services are the same as steps, but are executed first and are detached
	}
	for _, src := range append(pipeline.Services, pipeline.Steps...) { // combine: services+steps
		stepID := oshelp.Random()

		stepEnv := environ.Combine(envs, environ.Expand(convertStaticEnv(src.Environment)))
		stepSecrets := convertSecretEnv(src.Environment)

		var entrypoint []string
		var command []string
		var files []*lespec.File

		// set entrypoint if running on the host or if the container has commands
		if src.Image == "" || (src.Image != "" && len(src.Commands) > 0) {
			entrypoint = oshelp.GetEntrypoint(pipelinePlatform.OS)
		}

		// build the script of commands we will execute
		if len(src.Commands) > 0 {
			scriptToExecute := oshelp.GenScript(pipelinePlatform.OS, pipelinePlatform.Arch, src.Commands)
			scriptPath := oshelp.JoinPaths(pipelinePlatform.OS, pipelineRoot, "opt", oshelp.GetExt(pipelinePlatform.OS, stepID))

			files = []*lespec.File{
				{
					Path: scriptPath,
					Mode: 0700,
					Data: scriptToExecute,
				},
			}
			// the command is actually a file name where combined script for the step is located
			command = append(command, scriptPath)
		}

		// set working directory for the step and volume mount locations for steps that use an image
		var volumeMounts []*lespec.VolumeMount
		if src.Image != "" {
			haveImageSteps = true

			// add the volumes
			for _, v := range src.Volumes {
				volumeMounts = append(volumeMounts, &lespec.VolumeMount{Name: v.Name, Path: v.MountPath})
			}

			// mount the root drone directory in the container
			volumeMounts = append(volumeMounts, &lespec.VolumeMount{Name: "pipeline_root", Path: pipelineRoot})

			if len(src.Entrypoint) > 0 {
				entrypoint = src.Entrypoint

				// can't use both, entrypoint and commands... the entrypoint overrides the commands
				command = nil
				files = nil
			}
		}
		// appends the devices to the container def.
		var devices []*lespec.VolumeDevice
		for _, vol := range src.Devices {
			devices = append(devices, &lespec.VolumeDevice{
				Name:       vol.Name,
				DevicePath: vol.DevicePath,
			})
		}
		// appends the settings variables to the container definition.
		for key, value := range src.Settings {
			if value == nil {
				continue
			}

			// all settings are passed to the plugin env
			// variables, prefixed with PLUGIN_
			key = "PLUGIN_" + strings.ToUpper(key)

			// if the setting parameter is sources from the
			// secret we create a secret environment variable.
			if value.Secret != "" {
				stepSecrets = append(stepSecrets, &lespec.Secret{
					Name: value.Secret,
					Mask: true,
					Env:  key,
				})
			} else {
				// else if the setting parameter is opaque
				// we inject as a string-encoded environment
				// variable.
				stepEnv[key] = encoder.Encode(value.Value)
			}
		}

		// set the pipeline step run policy. steps run on success by default, but may be optionally configured to run on failure.
		runPolicy := runtime.RunOnSuccess
		if isRunAlways(src) {
			runPolicy = runtime.RunAlways
		} else if isRunOnFailure(src) {
			runPolicy = runtime.RunOnFailure
		}

		// if the pipeline step has unmet conditions the step is automatically skipped.
		if !src.When.Match(match) {
			runPolicy = runtime.RunNever
		}

		// set the pipeline failure policy. steps can choose to ignore the failure, or fail fast.
		errorPolicy := runtime.ErrFail
		switch src.Failure {
		case "ignore":
			errorPolicy = runtime.ErrIgnore
		case "fast", "fast-fail", "fail-fast":
			errorPolicy = runtime.ErrFailFast
		}

		// create the step
		spec.Steps = append(spec.Steps, &engine.Step{
			Step: lespec.Step{
				Command:      command,
				Detach:       src.Detach,
				Devices:      devices,
				DNS:          src.DNS,
				DNSSearch:    src.DNSSearch,
				Envs:         stepEnv,
				Entrypoint:   entrypoint,
				ExtraHosts:   src.ExtraHosts,
				Files:        files,
				ID:           stepID,
				Image:        src.Image,
				Name:         src.Name,
				Network:      src.Network,
				Networks:     nil, // not used by the runner
				PortBindings: src.PortBindings,
				Privileged:   src.Image != "", // all steps that use images, run in privileged mode
				Pull:         convertPullPolicy(src.Pull),
				Secrets:      stepSecrets,
				ShmSize:      int64(src.ShmSize),
				User:         src.User,
				Volumes:      volumeMounts,
				WorkingDir:   sourceDir,
			},
			DependsOn: src.DependsOn,
			ErrPolicy: errorPolicy,
			RunPolicy: runPolicy,
		})
	}
	var creds = []*drone.Registry{}
	// get registry credentials from registry plugins
	if c.Registry != nil {
		creds, _ = c.Registry.List(ctx, &registry.Request{
			Repo:  args.Repo,
			Build: args.Build,
		})
	}
	// get registry credentials from pull secrets
	for _, name := range pipeline.PullSecrets {
		if sec, ok := c.findSecret(ctx, args, name); ok {
			parsed, err := auths.ParseString(sec)
			if err == nil {
				creds = append(parsed, creds...)
			}
		}
	}

	for _, step := range spec.Steps {
		if step.Image == "" {
			continue
		}
		for _, cred := range creds {
			if leimage.MatchHostname(step.Image, cred.Address) {
				step.Auth = &lespec.Auth{
					Address:  cred.Address,
					Username: cred.Username,
					Password: cred.Password,
				}
				break
			}
		}
	}

	// labels
	systemLabels := labels.Combine(
		labels.FromRepo(args.Repo),
		labels.FromBuild(args.Build),
		labels.FromStage(args.Stage),
		labels.FromSystem(args.System),
		labels.WithTimeout(args.Repo),
	)

	// create network
	spec.Network = lespec.Network{
		ID:      oshelp.Random(),
		Labels:  systemLabels,
		Options: c.NetworkOpts,
	}

	// append global volumes and volume mounts to steps which have an image specified.
	for _, pair := range c.Volumes {
		src, dest, ro, err := resource.ParseVolume(pair)
		id := oshelp.Random()
		if err != nil {
			continue
		}
		volume := &lespec.Volume{
			HostPath: &lespec.VolumeHostPath{
				ID:       id,
				Name:     id,
				Path:     src,
				ReadOnly: ro,
			},
		}
		spec.Volumes = append(spec.Volumes, volume)
		for _, step := range spec.Steps {
			if step.Image == "" { // skip volume mounts on steps which don't have images
				continue
			}
			mount := &lespec.VolumeMount{
				Name: id,
				Path: dest,
			}
			step.Volumes = append(step.Volumes, mount)
		}
	}

	// create volumes
	for _, v := range pipeline.Volumes {
		if v.EmptyDir != nil {
			spec.Volumes = append(spec.Volumes, &lespec.Volume{
				EmptyDir: &lespec.VolumeEmptyDir{
					ID:     oshelp.Random(),
					Name:   v.Name,
					Labels: systemLabels,
				},
			})
		} else if v.HostPath != nil {
			spec.Volumes = append(spec.Volumes, &lespec.Volume{
				HostPath: &lespec.VolumeHostPath{
					ID:     oshelp.Random(),
					Name:   v.Name,
					Path:   v.HostPath.Path,
					Labels: systemLabels,
				},
			})
		}
	}
	if haveImageSteps {
		// source dir and script dir will be added as volumes only if there is at least on step that uses an image.
		spec.Volumes = append(spec.Volumes,
			&lespec.Volume{ // a mount for pipeline_root
				HostPath: &lespec.VolumeHostPath{
					ID:     "pipeline_root_" + oshelp.Random(),
					Name:   "pipeline_root",
					Path:   pipelineRoot,
					Labels: systemLabels,
				},
			})
	}

	// set step dependencies
	if !isGraph(spec) {
		configureSerial(spec)
	} else if !pipeline.Clone.Disable {
		configureCloneDeps(spec)
	} else if pipeline.Clone.Disable {
		removeCloneDeps(spec)
	}

	// set secret values
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
		return "", false
	}
	return found.Data, true
}

func createDirectories(pipelineOS, pipelineRoot string) (directories []*lespec.File, homeDir, sourceDir string) {
	homeRootDir := oshelp.JoinPaths(pipelineOS, pipelineRoot, "home")
	homeDir = oshelp.JoinPaths(pipelineOS, homeRootDir, "drone")

	droneDir := oshelp.JoinPaths(pipelineOS, pipelineRoot, "drone")
	sourceDir = oshelp.JoinPaths(pipelineOS, droneDir, "src")

	scriptDir := oshelp.JoinPaths(pipelineOS, pipelineRoot, "opt")

	directories = []*lespec.File{
		{Path: homeRootDir, Mode: 0700, IsDir: true},
		{Path: homeDir, Mode: 0700, IsDir: true},
		{Path: droneDir, Mode: 0700, IsDir: true},
		{Path: sourceDir, Mode: 0700, IsDir: true},
		{Path: scriptDir, Mode: 0700, IsDir: true},
	}

	return
}
