// Copyright 2020 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by the Polyform License
// that can be found in the LICENSE file.

package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/drone-runners/drone-runner-aws/command/config"

	"github.com/drone-runners/drone-runner-aws/internal/drivers"
	"github.com/drone-runners/drone-runner-aws/internal/lehelper"
	"github.com/drone-runners/drone-runner-aws/internal/oshelp"
	"github.com/drone/runner-go/environ"
	"github.com/drone/runner-go/logger"
	"github.com/drone/runner-go/pipeline/runtime"
	leapi "github.com/harness/lite-engine/api"
)

var (
	ErrorPoolNameEmpty  = errors.New("pool name is nil")
	ErrorPoolNotDefined = errors.New("pool not defined")
)

// Opts configures the Engine.
type Opts struct {
	Repopulate bool
}

// Engine implements a pipeline engine.
type Engine struct {
	opts        Opts
	poolManager *drivers.Manager
	config      *config.EnvConfig
}

// New returns a new engine.
func New(opts Opts, poolManager *drivers.Manager, envConfig *config.EnvConfig) (*Engine, error) {
	return &Engine{
		opts:        opts,
		poolManager: poolManager,
		config:      envConfig,
	}, nil
}

// Setup the pipeline environment.
func (e *Engine) Setup(ctx context.Context, specv runtime.Spec) error {
	spec := specv.(*Spec)

	poolName := spec.CloudInstance.PoolName
	manager := e.poolManager

	logr := logger.FromContext(ctx).
		WithField("func", "engine.Setup").
		WithField("pool", spec.CloudInstance.PoolName)

	if poolName == "" {
		logr.Errorln("pool name is missing")
		return ErrorPoolNameEmpty
	}

	// lets see if there is anything in the pool
	instance, err := manager.Provision(ctx, poolName, e.config.Runner.Name, e.config.Settings.LiteEnginePath)
	if err != nil {
		logr.WithError(err).Errorln("failed to provision an instance")
		return err
	}

	if instance.IsHibernated {
		instance, err = manager.StartInstance(ctx, poolName, instance.ID)
		if err != nil {
			logr.WithError(err).Errorln("failed to start an instance")
			return err
		}
	}

	logr = logr.
		WithField("ip", instance.Address).
		WithField("id", instance.ID)

	// now we have an instance, put the information in the spec
	spec.CloudInstance.PoolName = poolName
	spec.CloudInstance.ID = instance.ID
	spec.CloudInstance.IP = instance.Address

	if !manager.Exists(poolName) {
		logr.Errorln("pool does not exist")
		return ErrorPoolNotDefined
	}

	err = manager.Update(ctx, instance)
	if err != nil {
		logr.WithError(err).Errorln("failed to update instance")
		return err
	}
	// required for anka build where the port is dynamic
	client, err := lehelper.GetClient(instance, e.config.Runner.Name, instance.Port)
	if err != nil {
		logr.WithError(err).Errorln("failed to create LE client")
		return err
	}

	const timeoutSetup = 20 * time.Minute // TODO: Move to configuration

	// try the healthcheck api on the lite-engine until it responds ok
	logr.Traceln("running healthcheck and waiting for an ok response")
	healthResponse, err := client.RetryHealth(ctx, timeoutSetup)
	if err != nil {
		logr.WithError(err).Errorln("failed to call LE.RetryHealth")
		return err
	}

	logr.WithField("response", fmt.Sprintf("%+v", healthResponse)).
		Traceln("LE.RetryHealth check complete")
	setupRequest := &leapi.SetupRequest{
		Envs:      nil, // no global envs, envs are passed to each step individually
		Network:   spec.Network,
		Volumes:   spec.Volumes,
		Secrets:   nil,               // no global secrets, secrets are passed to each step individually
		LogConfig: leapi.LogConfig{}, // unused... I guess
		TIConfig:  leapi.TIConfig{},  // unused, CIE specific
		Files:     spec.Files,
	}

	// Currently the OSX m1 architecture does not enable nested virtualisation, so we disable docker.
	if instance.Platform.OS == oshelp.OSMac {
		b := false
		setupRequest.MountDockerSocket = &b
	}

	setupResponse, err := client.Setup(ctx, setupRequest)
	if err != nil {
		logr.WithError(err).Errorln("failed to call LE.Setup")
		return err
	}

	logr.WithField("response", fmt.Sprintf("%+v", setupResponse)).
		Traceln("LE.Setup complete")

	return nil
}

// Destroy the pipeline environment.
func (e *Engine) Destroy(ctx context.Context, specv runtime.Spec) error {
	const destroyTimeout = time.Second * 5 // HACK: this timeout delays deleting the instance to ensure there is enough time to stream the logs.
	time.Sleep(destroyTimeout)

	spec := specv.(*Spec)

	poolName := spec.CloudInstance.PoolName
	poolMngr := e.poolManager

	instanceID := spec.CloudInstance.ID
	instanceIP := spec.CloudInstance.IP

	logr := logger.FromContext(ctx).
		WithField("func", "engine.Destroy").
		WithField("pool", poolName).
		WithField("id", instanceID).
		WithField("ip", instanceIP)

	logr.Infof("destroying instance %s", instanceID)

	if err := poolMngr.Destroy(ctx, poolName, instanceID); err != nil {
		logr.WithError(err).Errorln("cannot destroy the instance")
		return err
	}
	logr.Traceln("destroyed instance")

	return nil
}

// Run runs the pipeline step.
func (e *Engine) Run(ctx context.Context, specv runtime.Spec, stepv runtime.Step, output io.Writer) (*runtime.State, error) {
	spec := specv.(*Spec)
	step := stepv.(*Step)

	poolName := spec.CloudInstance.PoolName
	instanceID := spec.CloudInstance.ID
	instanceIP := spec.CloudInstance.IP

	logr := logger.FromContext(ctx).
		WithField("func", "engine.Run").
		WithField("pool", poolName).
		WithField("step_id", step.Name).
		WithField("id", instanceID).
		WithField("ip", instanceIP)

	instance, err := e.poolManager.Find(ctx, instanceID)
	if err != nil {
		logr.WithError(err).Errorln("cannot find instance")
		return nil, err
	}
	client, err := lehelper.GetClient(instance, e.config.Runner.Name, instance.Port)
	if err != nil {
		logr.WithError(err).Errorln("failed to create LE client")
		return nil, err
	}

	const timeoutStep = 4 * time.Hour // TODO: Move to configuration

	secretEnvs := make(map[string]string, len(step.Secrets))
	for _, secret := range step.Secrets {
		secretEnvs[secret.Env] = string(secret.Data)
	}

	// TODO: This code repacks the step data. This is unfortunate implementation in LE. Step should be embedded in StartStepRequest. Should be improved.
	req := &leapi.StartStepRequest{
		Auth:         step.Auth,
		CPUPeriod:    step.CPUPeriod,
		CPUQuota:     step.CPUQuota,
		CPUShares:    step.CPUShares,
		CPUSet:       step.CPUSet,
		Files:        step.Files,
		Detach:       step.Detach,
		Devices:      step.Devices,
		DNS:          step.DNS,
		DNSSearch:    step.DNSSearch,
		Envs:         environ.Combine(step.Envs, secretEnvs),
		ExtraHosts:   step.ExtraHosts,
		ID:           step.ID,
		IgnoreStdout: step.IgnoreStdout,
		IgnoreStderr: step.IgnoreStdout,
		Image:        step.Image,
		Kind:         leapi.Run,
		Labels:       step.Labels,
		LogKey:       step.ID,
		LogDrone:     true, // must be true for the logging to work
		MemSwapLimit: step.MemSwapLimit,
		MemLimit:     step.MemLimit,
		Name:         step.Name,
		Network:      step.Network,
		Networks:     step.Networks,
		OutputVars:   nil, // not used by Drone
		PortBindings: step.PortBindings,
		Privileged:   step.Privileged,
		Pull:         step.Pull,
		Run: leapi.RunConfig{
			Command:    step.Command,
			Entrypoint: step.Entrypoint,
		},
		RunTest:    leapi.RunTestConfig{},
		Secrets:    nil, // not used by Drone
		ShmSize:    step.ShmSize,
		TestReport: leapi.TestReport{},
		Timeout:    int(timeoutStep.Seconds()),
		User:       step.User,
		Volumes:    step.Volumes,
		WorkingDir: step.WorkingDir,
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func(ctx context.Context) {
		var totalWritten counterWriter
		w := io.MultiWriter(output, &totalWritten)

		defer func() {
			wg.Done()
			logr.WithField("len", int(totalWritten)).Traceln("finished streaming step output")
		}()
		logr.Traceln("streaming step output")

		streamErr := client.GetStepLogOutput(ctx, &leapi.StreamOutputRequest{ID: req.ID, Offset: 0}, w)
		if streamErr != nil {
			if step.Detach && errors.Is(streamErr, context.Canceled) {
				logr.WithError(streamErr).Traceln("aborted detached step output streaming")
			} else if totalWritten == 0 {
				logr.WithError(streamErr).Errorln("failed to stream step output")
			} else {
				logr.WithError(streamErr).Warnln("failed to finish step output streaming")
			}
		}
	}(ctx)

	// Currently the OSX m1 architecture does not enable nested virtualisation, so we disable docker.
	if instance.Platform.OS == oshelp.OSMac {
		b := false
		req.MountDockerSocket = &b
	}
	startStepResponse, err := client.StartStep(ctx, req)
	if err != nil {
		logr.WithError(err).Errorln("failed to start step")
		return nil, err
	}

	logr.WithField("startStepResponse", startStepResponse).
		Traceln("LE.StartStep complete")

	pollResponse, err := client.RetryPollStep(ctx, &leapi.PollStepRequest{ID: req.ID}, timeoutStep)
	if err != nil {
		logr.WithError(err).Errorln("failed to poll step result")
		return nil, err
	}

	logr.WithField("pollResponse", pollResponse).
		Traceln("completed LE.RetryPollStep")

	wg.Wait()

	spec.OutputVars = pollResponse.Outputs

	state := &runtime.State{
		ExitCode:  pollResponse.ExitCode,
		Exited:    pollResponse.Exited,
		OOMKilled: pollResponse.OOMKilled,
	}

	return state, nil
}

type counterWriter int

func (q *counterWriter) Write(data []byte) (int, error) {
	*q += counterWriter(len(data))
	return len(data), nil
}
