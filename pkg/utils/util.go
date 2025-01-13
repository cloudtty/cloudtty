/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"text/template"

	"k8s.io/klog/v2"

	"github.com/pkg/errors"
)

func LoadYamlTemplate(fname string) (str string, err error) {
	// function ioutil.ReadFile is deprecated as of Go 1.16, it is simply calls os.ReadFile.
	b, err := os.ReadFile(fname)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ParseTemplate validates and parses passed as argument template.
func ParseTemplate(strtmpl string, obj interface{}) ([]byte, error) {
	var buf bytes.Buffer
	tmpl, err := template.New("template").Parse(strtmpl)
	if err != nil {
		return nil, errors.Wrap(err, "error when parsing template")
	}
	err = tmpl.Execute(&buf, obj)
	if err != nil {
		return nil, errors.Wrap(err, "error when executing template")
	}
	return buf.Bytes(), nil
}

// GetCurrentNS fetch namespace the current pod running in.
func GetCurrentNSOrDefault() string {
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		return ns
	}
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if ns := strings.TrimSpace(string(data)); len(ns) > 0 {
			return ns
		}
	} else {
		klog.ErrorS(err, "Failed to get namespace where pods running in")
	}
	return "default"
}

func MapToJSONString(m map[string]string) (string, error) {
	result, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func JSONStringToMap(jsonString string) (map[string]string, error) {
	var result map[string]string
	err := json.Unmarshal([]byte(jsonString), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
