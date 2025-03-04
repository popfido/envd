// Copyright 2022 The envd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ir

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/moby/buildkit/client/llb"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/tensorchord/envd/pkg/config"
	"github.com/tensorchord/envd/pkg/flag"
	"github.com/tensorchord/envd/pkg/progress/compileui"
	"github.com/tensorchord/envd/pkg/types"
	"github.com/tensorchord/envd/pkg/version"
)

func NewGraph() *Graph {
	runtimeGraph := RuntimeGraph{
		RuntimeCommands: make(map[string]string),
		RuntimeEnviron:  make(map[string]string),
	}
	langVersion := languageVersionDefault
	return &Graph{
		OS: osDefault,
		Language: Language{
			Name:    languageDefault,
			Version: &langVersion,
		},
		CUDA:    nil,
		CUDNN:   nil,
		NumGPUs: -1,

		PyPIPackages:   []string{},
		RPackages:      []string{},
		JuliaPackages:  []string{},
		SystemPackages: []string{},
		Exec:           []string{},
		Shell:          shellBASH,
		RuntimeGraph:   runtimeGraph,
	}
}

var DefaultGraph = NewGraph()

func GPUEnabled() bool {
	return DefaultGraph.GPUEnabled()
}

func NumGPUs() int {
	return DefaultGraph.NumGPUs
}

func Compile(ctx context.Context, envName string, pub string) (*llb.Definition, error) {
	w, err := compileui.New(ctx, os.Stdout, "auto")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create compileui")
	}
	DefaultGraph.Writer = w
	DefaultGraph.EnvironmentName = envName
	DefaultGraph.PublicKeyPath = pub

	uid, gid, err := getUIDGID()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get uid/gid")
	}
	state, err := DefaultGraph.Compile(uid, gid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile")
	}
	// TODO(gaocegege): Support multi platform.
	def, err := state.Marshal(ctx, llb.LinuxAmd64)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal the llb definition")
	}
	return def, nil
}

func Labels() (map[string]string, error) {
	return DefaultGraph.Labels()
}

func ExposedPorts() (map[string]struct{}, error) {
	return DefaultGraph.ExposedPorts()
}

func CompileEntrypoint(buildContextDir string) ([]string, error) {
	return DefaultGraph.GetEntrypoint(buildContextDir)
}

func CompileEnviron() []string {
	return DefaultGraph.EnvString()
}

func (g Graph) GPUEnabled() bool {
	return g.CUDA != nil
}

func (g Graph) Labels() (map[string]string, error) {
	labels := make(map[string]string)
	str, err := json.Marshal(g.SystemPackages)
	if err != nil {
		return nil, err
	}
	labels[types.ImageLabelAPT] = string(str)
	str, err = json.Marshal(g.PyPIPackages)
	if err != nil {
		return nil, err
	}
	labels[types.ImageLabelPyPI] = string(str)
	str, err = json.Marshal(g.RPackages)
	if err != nil {
		return nil, err
	}
	labels[types.ImageLabelR] = string(str)
	if g.GPUEnabled() {
		labels[types.ImageLabelGPU] = "true"
		labels[types.ImageLabelCUDA] = *g.CUDA
		if g.CUDNN != nil {
			labels[types.ImageLabelCUDNN] = *g.CUDNN
		}
	}
	labels[types.ImageLabelVendor] = types.ImageVendorEnvd

	return labels, nil
}

func (g Graph) ExposedPorts() (map[string]struct{}, error) {
	ports := make(map[string]struct{})

	// do not expose ports for custom images
	if g.Image != nil {
		return ports, nil
	}

	ports[fmt.Sprintf("%d/tcp", config.SSHPortInContainer)] = struct{}{}
	if g.JupyterConfig != nil {
		ports[fmt.Sprintf("%d/tcp", config.JupyterPortInContainer)] = struct{}{}
	}
	if g.RStudioServerConfig != nil {
		ports[fmt.Sprintf("%d/tcp", config.RStudioServerPortInContainer)] = struct{}{}
	}

	if g.RuntimeExpose != nil && len(g.RuntimeExpose) > 0 {
		for _, item := range g.RuntimeExpose {
			ports[fmt.Sprintf("%d/tcp", item.EnvdPort)] = struct{}{}
		}
	}

	return ports, nil
}

func (g Graph) EnvString() []string {
	var envs []string
	for k, v := range g.RuntimeEnviron {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}
	return envs
}

func (g Graph) DefaultCacheImporter() (*string, error) {
	// The base remote cache should work for all languages.
	res := fmt.Sprintf(
		"type=registry,ref=docker.io/%s/python-cache:envd-%s",
		viper.GetString(flag.FlagDockerOrganization),
		version.GetVersionForImageTag())
	return &res, nil
}

func (g Graph) GetEntrypoint(buildContextDir string) ([]string, error) {
	if g.Image != nil {
		return g.Entrypoint, nil
	}

	ep := []string{
		"tini",
		"--",
		"bash",
		"-c",
	}

	template := `set -e
/var/envd/bin/envd-ssh --authorized-keys %s --port %d --shell %s &
%s
wait -n`

	// Generate jupyter and rstudio server commands.
	var customCmd strings.Builder
	workingDir := filepath.Join("/home/envd", filepath.Base(buildContextDir))
	if g.RuntimeDaemon != nil {
		for _, command := range g.RuntimeDaemon {
			customCmd.WriteString(fmt.Sprintf("%s &\n", strings.Join(command, " ")))
		}
	}
	if g.JupyterConfig != nil {
		jupyterCmd := g.generateJupyterCommand(workingDir)
		customCmd.WriteString(strings.Join(jupyterCmd, " "))
		customCmd.WriteString("\n")
	}
	if g.RStudioServerConfig != nil {
		rstudioCmd := g.generateRStudioCommand(workingDir)
		customCmd.WriteString(strings.Join(rstudioCmd, " "))
		customCmd.WriteString("\n")
	}

	cmd := fmt.Sprintf(template,
		config.ContainerAuthorizedKeysPath,
		config.SSHPortInContainer, g.Shell, customCmd.String())
	ep = append(ep, cmd)

	logrus.WithField("entrypoint", ep).Debug("generate entrypoint")
	return ep, nil
}

func (g Graph) Compile(uid, gid int) (llb.State, error) {
	g.uid = uid

	// TODO(gaocegege): Remove the hack for https://github.com/tensorchord/envd/issues/370
	g.gid = 1001
	logrus.WithFields(logrus.Fields{
		"uid": g.uid,
		"gid": g.gid,
	}).Debug("compile LLB")

	// TODO(gaocegege): Support more OS and langs.
	base, err := g.compileBase()
	if err != nil {
		return llb.State{}, errors.Wrap(err, "failed to get the base image")
	}
	aptStage := g.compileUbuntuAPT(base)
	var merged llb.State
	// Use custom logic when image is specified.
	if g.Image != nil {
		merged, err = g.compileCustomPython(aptStage)
		if err != nil {
			return llb.State{}, errors.Wrap(err, "failed to compile custom python image")
		}
	} else {
		switch g.Language.Name {
		case "r":
			merged, err = g.compileRLang(aptStage)
			if err != nil {
				return llb.State{}, errors.Wrap(err, "failed to compile r language")
			}
		case "python":
			merged, err = g.compilePython(aptStage)
			if err != nil {
				return llb.State{}, errors.Wrap(err, "failed to compile python")
			}
		case "julia":
			merged, err = g.compileJulia(aptStage)
			if err != nil {
				return llb.State{}, errors.Wrap(err, "failed to compile julia")
			}
		}
	}

	prompt := g.compilePrompt(merged)
	copy := g.compileCopy(prompt)
	// TODO(gaocegege): Support order-based exec.
	run := g.compileRun(copy)
	finalStage, err := g.compileGit(run)
	if err != nil {
		return llb.State{}, errors.Wrap(err, "failed to compile git")
	}
	g.Writer.Finish()
	return finalStage, nil
}
