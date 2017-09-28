//
// Copyright (c) 2017 TODO figure out the license person
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	logging "github.com/op/go-logging"
	"github.com/openshift/ansible-service-broker/pkg/apb"
)

const ecrName = "ecr" // TODO is there a better name?

// ECRAdapter - Amazon EC2 Container Registry Adapter
type ECRAdapter struct {
	Config Configuration
	Log    *logging.Logger
	ecr    *ecr.ECR
}

// connect creates an instance of the ECR service if it is not already created
func (r *ECRAdapter) connect() error {
	// If the ECR already exists no need to do anything
	if r.ecr != nil {
		return nil
	}

	// Make a new session with the default config
	sess, err := session.NewSession()
	if err != nil {
		r.Log.Errorf("Error creating AWS session - %s", err)
		return err
	}

	r.ecr = ecr.New(sess)

	return nil

}

// RegistryName - Retrieve the registry name
func (r *ECRAdapter) RegistryName() string {
	return ecrName
}

// GetImageNames - retrieve the images
func (r *ECRAdapter) GetImageNames() ([]string, error) {
	r.Log.Debug("ECRAdapter::GetImages")
	r.Log.Debug("BundleSpecLabel: %s", BundleSpecLabel)
	r.Log.Debug("Loading image list for org: [ %s ]", r.Config.Org)

	imageNames := make([]string, 0)
	err := r.connect()
	if err != nil {
		return imageNames, err
	}

	err = r.ecr.DescribeRepositoriesPages(params,
		func(page *DescribeRepositoriesOutput, lastPage bool) bool {

			for _, r := range page.Repositories {
				imageNames = append(imageNames, *r.RepositoryName)
			}

			return lastPage
		})

	// check to see if the context had an error
	if err != nil {
		r.Log.Errorf("encountered an error while loading images, we may not have all the images in the catalog - %v", err)
	}

	return imageNames, err
}

// FetchSpecs - retrieve the spec for the image names.
func (r *ECRAdapter) FetchSpecs(imageNames []string) ([]*apb.Spec, error) {
	specs := []*apb.Spec{}
	err = r.connect()

	if err != nil {
		return specs, err
	}

	for _, imageName := range imageNames {
		spec, err := r.loadSpec(imageName)
		if err != nil {
			r.Log.Errorf("unable to retrieve spec data for image - %v", err)
			return specs, err
		}
		if spec != nil {
			specs = append(specs, spec)
		}
	}
	return specs, nil
}

func (r *ECRAdapter) loadSpec(imageName string) (*apb.Spec, error) {
	if r.Config.Tag == "" {
		r.Config.Tag = "latest"
	}

	resp, err := r.ecr.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return nil, err
	}

	// TODO this url is a shot in the dark basically
	manifestUrl := fmt.Sprintf("%s/v2/%s/manifests/%s", *resp.AuthorizationData.ProxyEndpoint, imageName, r.Config.Tag)

	req, err := http.NewRequest("GET", manifestUrl, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", *resp.AuthorizationData.AuthorizationToken))
	return imageToSpec(r.Log, req, r.Config.Tag)
}
