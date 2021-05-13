/*
Copyright IBM Corporation 2020

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

package starlark

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/konveyor/move2kube/internal/common"
	starcommon "github.com/konveyor/move2kube/internal/starlark/common"
	"github.com/konveyor/move2kube/internal/starlark/types"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var (
	stripHelmQuotesRegex = regexp.MustCompile(`'({{.+}})'`)
)

// WriteResources writes out k8s resources to a given directory.
// It will create the output directory if it doesn't exist.
func WriteResources(k8sResources []types.K8sResourceT, outputPath string) ([]string, error) {
	log.Trace("start WriteResources")
	defer log.Trace("end WriteResources")
	if err := os.MkdirAll(outputPath, common.DefaultDirectoryPermission); err != nil {
		return nil, err
	}
	filesWritten := []string{}
	for _, k8sResource := range k8sResources {
		filename, err := getFilename(k8sResource)
		if err != nil {
			continue
		}
		fileOutputPath := filepath.Join(outputPath, filename)
		if err := WriteResource(k8sResource, fileOutputPath); err != nil {
			continue
		}
		filesWritten = append(filesWritten, fileOutputPath)
	}
	return filesWritten, nil
}

func getFilename(k8sResource types.K8sResourceT) (string, error) {
	log.Trace("start getFilename")
	defer log.Trace("end getFilename")
	kind, _, name, err := starcommon.GetInfoFromK8sResource(k8sResource)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s.yaml", name, strings.ToLower(kind)), nil
}

// WriteResource writes out a k8s resource to a given file path.
func WriteResource(k8sResource types.K8sResourceT, outputPath string) error {
	log.Trace("start WriteResource")
	defer log.Trace("end WriteResource")
	yamlBytes, err := yaml.Marshal(k8sResource)
	if err != nil {
		log.Error("Error while Encoding object")
		return err
	}
	return ioutil.WriteFile(outputPath, yamlBytes, common.DefaultFilePermission)
}

// WriteResourcesStripQuotes is like the WriteResources but strips quotes around Helm templates
func WriteResourcesStripQuotes(k8sResources []types.K8sResourceT, outputPath string) ([]string, error) {
	log.Trace("start WriteResourcesStripQuotes")
	defer log.Trace("end WriteResourcesStripQuotes")
	if err := os.MkdirAll(outputPath, common.DefaultDirectoryPermission); err != nil {
		return nil, err
	}
	filesWritten := []string{}
	for _, k8sResource := range k8sResources {
		filename, err := getFilename(k8sResource)
		if err != nil {
			continue
		}
		fileOutputPath := filepath.Join(outputPath, filename)
		if err := WriteResourceStripQuotes(k8sResource, fileOutputPath); err != nil {
			continue
		}
		filesWritten = append(filesWritten, fileOutputPath)
	}
	return filesWritten, nil
}

// WriteResourcesStripQuotesPreservingPaths is like the WriteResourcesStripQuotes but preserves the folder structure
func WriteResourcesStripQuotesPreservingPaths(k8sResources map[string][]types.K8sResourceT, outputPath string) ([]string, error) {
	log.Trace("start WriteResourcesStripQuotesPreservingPaths")
	defer log.Trace("end WriteResourcesStripQuotesPreservingPaths")
	if err := os.MkdirAll(outputPath, common.DefaultDirectoryPermission); err != nil {
		return nil, err
	}
	filesWritten := []string{}
	for fileOutputPath, resources := range k8sResources {
		parentDir := filepath.Base(fileOutputPath)
		if err := os.MkdirAll(parentDir, common.DefaultDirectoryPermission); err != nil {
			log.Errorf("failed to create the output directory at path %s . Error: %q", parentDir, err)
			continue
		}
		for _, resource := range resources {
			if err := WriteResourceStripQuotesAndAppendToFile(resource, fileOutputPath); err != nil {
				log.Errorf("failed to create the output ks8 yaml at path %s . Error: %q", fileOutputPath, err)
				continue
			}
			filesWritten = append(filesWritten, fileOutputPath)
		}
	}
	return filesWritten, nil
}

// WriteResourceStripQuotes is like WriteResource but strips quotes around Helm templates
func WriteResourceStripQuotes(k8sResource types.K8sResourceT, outputPath string) error {
	log.Trace("start WriteResourceStripQuotes")
	defer log.Trace("end WriteResourceStripQuotes")
	yamlBytes, err := yaml.Marshal(k8sResource)
	if err != nil {
		log.Error("Error while Encoding object")
		return err
	}
	strippedYamlBytes := stripHelmQuotesRegex.ReplaceAll(yamlBytes, []byte("$1"))
	return ioutil.WriteFile(outputPath, strippedYamlBytes, common.DefaultFilePermission)
}

// WriteResourceAppendToFile is like WriteResource but appends to the file
func WriteResourceAppendToFile(k8sResource types.K8sResourceT, outputPath string) error {
	log.Trace("start WriteResourceAppendToFile")
	defer log.Trace("end WriteResourceAppendToFile")
	yamlBytes, err := yaml.Marshal(k8sResource)
	if err != nil {
		log.Error("Error while Encoding object")
		return err
	}
	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, common.DefaultFilePermission)
	if err != nil {
		return fmt.Errorf("failed to open the file at path %s for creating/appending. Error: %q", outputPath, err)
	}
	defer f.Close()
	if _, err := f.Write([]byte("\n---\n" + string(yamlBytes) + "\n...\n")); err != nil {
		return fmt.Errorf("failed to write to the file at path %s . Error: %q", outputPath, err)
	}
	return f.Close()
}

// WriteResourceStripQuotesAndAppendToFile is like WriteResource but strips quotes around Helm templates and appends to file
func WriteResourceStripQuotesAndAppendToFile(k8sResource types.K8sResourceT, outputPath string) error {
	log.Trace("start WriteResourceStripQuotesAndAppendToFile")
	defer log.Trace("end WriteResourceStripQuotesAndAppendToFile")
	yamlBytes, err := yaml.Marshal(k8sResource)
	if err != nil {
		log.Error("Error while Encoding object")
		return err
	}
	strippedYamlBytes := stripHelmQuotesRegex.ReplaceAll(yamlBytes, []byte("$1"))
	// If the file doesn't exist, create it, or append to the file
	f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, common.DefaultFilePermission)
	if err != nil {
		return fmt.Errorf("failed to open the file at path %s for creating/appending. Error: %q", outputPath, err)
	}
	defer f.Close()
	if _, err := f.Write([]byte("\n---\n" + string(strippedYamlBytes) + "\n...\n")); err != nil {
		return fmt.Errorf("failed to write to the file at path %s . Error: %q", outputPath, err)
	}
	return f.Close()
}
