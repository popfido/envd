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

package install

import (
	"path/filepath"

	"github.com/sirupsen/logrus"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/tensorchord/envd/pkg/lang/frontend/starlark/builtin"
	"github.com/tensorchord/envd/pkg/lang/ir"
)

var (
	logger = logrus.WithField("frontend", "starlark")
)

var Module = &starlarkstruct.Module{
	Name: "install",
	Members: starlark.StringDict{
		"python_packages":   starlark.NewBuiltin(rulePyPIPackage, ruleFuncPyPIPackage),
		"r_packages":        starlark.NewBuiltin(ruleRPackage, ruleFuncRPackage),
		"system_packages":   starlark.NewBuiltin(ruleSystemPackage, ruleFuncSystemPackage),
		"cuda":              starlark.NewBuiltin(ruleCUDA, ruleFuncCUDA),
		"vscode_extensions": starlark.NewBuiltin(ruleVSCode, ruleFuncVSCode),
		"conda_packages":    starlark.NewBuiltin(ruleConda, ruleFuncConda),
		"julia_packages":    starlark.NewBuiltin(ruleJulia, ruleFuncJulia),
	},
}

func ruleFuncPyPIPackage(thread *starlark.Thread, _ *starlark.Builtin,
	args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name *starlark.List
	var requirementsFile starlark.String

	if err := starlark.UnpackArgs(rulePyPIPackage, args, kwargs,
		"name?", &name, "requirements?", &requirementsFile); err != nil {
		return nil, err
	}

	nameList := []string{}
	if name != nil {
		for i := 0; i < name.Len(); i++ {
			nameList = append(nameList, name.Index(i).(starlark.String).GoString())
		}
	}

	requirementsFileStr := requirementsFile.GoString()

	logger.Debugf("rule `%s` is invoked, name=%v, requirements=%s",
		rulePyPIPackage, nameList, requirementsFileStr)

	err := ir.PyPIPackage(nameList, requirementsFileStr)
	return starlark.None, err
}

func ruleFuncRPackage(thread *starlark.Thread, _ *starlark.Builtin,
	args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name *starlark.List

	if err := starlark.UnpackArgs(ruleRPackage,
		args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	nameList := []string{}
	if name != nil {
		for i := 0; i < name.Len(); i++ {
			nameList = append(nameList, name.Index(i).(starlark.String).GoString())
		}
	}

	logger.Debugf("rule `%s` is invoked, name=%v", ruleRPackage, nameList)
	ir.RPackage(nameList)

	return starlark.None, nil
}

func ruleFuncJulia(thread *starlark.Thread, _ *starlark.Builtin,
	args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name *starlark.List

	if err := starlark.UnpackArgs(ruleJulia,
		args, kwargs, "name", &name); err != nil {
		return nil, err
	}

	nameList := []string{}
	if name != nil {
		for i := 0; i < name.Len(); i++ {
			nameList = append(nameList, name.Index(i).(starlark.String).GoString())
		}
	}

	logger.Debugf("rule `%s` is invoked, name=%v", ruleJulia, nameList)
	ir.JuliaPackage(nameList)

	return starlark.None, nil
}

func ruleFuncSystemPackage(thread *starlark.Thread, _ *starlark.Builtin,
	args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name *starlark.List

	if err := starlark.UnpackArgs(ruleSystemPackage,
		args, kwargs, "name?", &name); err != nil {
		return nil, err
	}

	nameList := []string{}
	if name != nil {
		for i := 0; i < name.Len(); i++ {
			nameList = append(nameList, name.Index(i).(starlark.String).GoString())
		}
	}

	logger.Debugf("rule `%s` is invoked, name=%v", ruleSystemPackage, nameList)
	ir.SystemPackage(nameList)

	return starlark.None, nil
}

func ruleFuncCUDA(thread *starlark.Thread, _ *starlark.Builtin,
	args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var version, cudnn starlark.String

	if err := starlark.UnpackArgs(ruleCUDA, args, kwargs,
		"version?", &version, "cudnn?", &cudnn); err != nil {
		return nil, err
	}

	versionStr := version.GoString()
	cudnnStr := cudnn.GoString()

	logger.Debugf("rule `%s` is invoked, version=%s, cudnn=%s",
		ruleCUDA, versionStr, cudnnStr)
	ir.CUDA(versionStr, cudnnStr)

	return starlark.None, nil
}

func ruleFuncVSCode(thread *starlark.Thread, _ *starlark.Builtin,
	args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var plugins *starlark.List

	if err := starlark.UnpackArgs(ruleVSCode,
		args, kwargs, "name", &plugins); err != nil {
		return nil, err
	}

	pluginList := []string{}
	if plugins != nil {
		for i := 0; i < plugins.Len(); i++ {
			pluginList = append(pluginList, plugins.Index(i).(starlark.String).GoString())
		}
	}

	logger.Debugf("rule `%s` is invoked, plugins=%v", ruleVSCode, pluginList)
	if err := ir.VSCodePlugins(pluginList); err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}

func ruleFuncConda(thread *starlark.Thread, _ *starlark.Builtin,
	args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name, channel *starlark.List
	var envFile starlark.String

	if err := starlark.UnpackArgs(ruleConda,
		args, kwargs, "name?", &name, "channel?", &channel, "env_file?", &envFile); err != nil {
		return nil, err
	}

	nameList := []string{}
	if name != nil {
		for i := 0; i < name.Len(); i++ {
			nameList = append(nameList, name.Index(i).(starlark.String).GoString())
		}
	}

	channelList := []string{}
	if channel != nil {
		for i := 0; i < channel.Len(); i++ {
			channelList = append(channelList, channel.Index(i).(starlark.String).GoString())
		}
	}
	var path *string = nil
	envFileStr := envFile.GoString()
	if envFileStr != "" {
		buildContextDir := starlark.Universe[builtin.BuildContextDir]
		buildContextDirStr := buildContextDir.(starlark.String).GoString()
		buf := filepath.Join(buildContextDirStr, envFileStr)
		path = &buf
	}

	logger.Debugf("rule `%s` is invoked, name=%v, channel=%v, env_file=%s", ruleConda, nameList, channelList, envFileStr)
	if err := ir.CondaPackage(nameList, channelList, path); err != nil {
		return starlark.None, err
	}

	return starlark.None, nil
}
