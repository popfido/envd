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

package vscode

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/tensorchord/envd/pkg/home"
	"github.com/tensorchord/envd/pkg/unzip"
	"github.com/tensorchord/envd/pkg/util/fileutil"
)

type Client interface {
	DownloadOrCache(plugin Plugin) (bool, error)
	PluginPath(p Plugin) string
}

type generalClient struct {
}

func NewClient() Client {
	return &generalClient{}
}

func (c generalClient) PluginPath(p Plugin) string {
	return fmt.Sprintf("%s.%s-%s/extension/", p.Publisher, p.Extension, p.Version)
}

func unzipPath(p Plugin) string {
	return fmt.Sprintf("%s/%s.%s-%s", home.GetManager().CacheDir(),
		p.Publisher, p.Extension, p.Version)
}

// DownloadOrCache downloads or cache the plugin.
// If the plugin is already downloaded, it returns true.
func (c generalClient) DownloadOrCache(p Plugin) (bool, error) {
	url := fmt.Sprintf(vscodePackageURLTemplate,
		p.Publisher, p.Publisher, p.Extension, p.Version)

	filename := fmt.Sprintf("%s/%s.%s-%s.vsix", home.GetManager().CacheDir(),
		p.Publisher, p.Extension, p.Version)
	logger := logrus.WithFields(logrus.Fields{
		"publisher": p.Publisher,
		"extension": p.Extension,
		"version":   p.Version,
		"url":       url,
		"file":      filename,
	})
	if ok, err := fileutil.FileExists(filename); err != nil {
		return false, err
	} else if ok {
		logger.Debug("vscode plugin is cached")
		return true, nil
	}
	out, err := os.Create(filename)

	if err != nil {
		return false, err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return false, err
	}
	logger.Debugf("downloading vscode plugin")

	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return false, err
	}

	_, err = unzip.Unzip(filename, unzipPath(p))
	if err != nil {
		return false, errors.Wrap(err, "failed to unzip")
	}

	return false, nil
}

func ParsePlugin(p string) (*Plugin, error) {
	indexPublisher := strings.Index(p, ".")
	if indexPublisher == -1 {
		return nil, errors.New("invalid publisher")
	}
	publisher := p[:indexPublisher]

	indexExtension := strings.LastIndex(p[indexPublisher:], "-")
	if indexExtension == -1 {
		return nil, errors.New("invalid extension")
	}

	indexExtension = indexPublisher + indexExtension
	extension := p[indexPublisher+1 : indexExtension]
	version := p[indexExtension+1:]
	logrus.WithFields(logrus.Fields{
		"publisher": publisher,
		"extension": extension,
		"version":   version,
	}).Debug("vscode plugin is parsed")
	return &Plugin{
		Publisher: publisher,
		Extension: extension,
		Version:   version,
	}, nil
}