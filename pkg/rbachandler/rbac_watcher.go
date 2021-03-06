// Copyright © 2019 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rbachandler

import (
	"fmt"
	"time"

	"github.com/goharbor/harbor/src/jobservice/logger"
	"github.com/goph/emperror"
	"github.com/goph/logur"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WatchSATokens watch created token
func WatchSATokens(config *Config, logger logur.Logger) {
	rbacHandler, err := NewRBACHandler(config.KubeConfig, logger)
	if err != nil {
		logger.Error(err.Error(), nil)
	}
	func() {
		ticker := time.NewTicker(1 * time.Minute)
		for t := range ticker.C {
			if err := rbacHandler.evaluateLabeledSecrets(t); err != nil {
				logger.Error(err.Error(), nil)
			}
		}
	}()
}

func (rh *RBACHandler) evaluateLabeledSecrets(t time.Time) error {
	labelSelect := fmt.Sprintf("%s=%s", defautlLabelKey, defaultLabel[defautlLabelKey])
	listOptions := metav1.ListOptions{
		LabelSelector: labelSelect,
	}
	secretList, err := rh.coreClientSet.Secrets("default").List(listOptions)
	if err != nil {
		return emperror.Wrap(err, "getting labeled secrets failed")
	}
	if len(secretList.Items) > 0 {
		for _, sec := range secretList.Items {
			logger.Debug("checking secret", map[string]interface{}{"secname": sec.Name})
			if rh.checkTTL(sec.Name) != nil {
				return err
			}
		}
	}
	return nil
}

func (rh *RBACHandler) checkTTL(secretName string) error {
	secret, err := rh.coreClientSet.Secrets("default").Get(secretName, metav1.GetOptions{})
	if err != nil {
		return emperror.With(err, "secret_name", secretName)
	}

	deleteTime, err := time.Parse(time.RFC3339, secret.GetAnnotations()["banzaicloud.io/timetolive"])
	if err != nil {
		return emperror.With(err, "delete_time", secret.GetAnnotations()["banzaicloud.io/timetolive"])
	}
	if deleteTime.Before(time.Now()) {
		err := rh.coreClientSet.Secrets("default").Delete(secretName, &metav1.DeleteOptions{})
		if err != nil {
			return emperror.WrapWith(err, "create secret failed", "secretName", secretName)
		}
	}
	return nil
}
