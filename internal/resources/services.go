/*

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

package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var svcSelectorKey = "samba-operator.samba.org/service"

func newServiceForSmb(planner *sharePlanner, ns string) *corev1.Service {
	labels := labelsForSmbServer(planner.instanceName())
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planner.instanceName(),
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: toServiceType(planner.serviceType()),
			Ports: []corev1.ServicePort{{
				Name:     "smb",
				Protocol: corev1.ProtocolTCP,
				Port:     445,
			}},
			Selector: map[string]string{
				svcSelectorKey: labels[svcSelectorKey],
			},
		},
	}
}

func toServiceType(s string) corev1.ServiceType {
	svcType := corev1.ServiceType(s)
	switch svcType {
	case corev1.ServiceTypeClusterIP:
	case corev1.ServiceTypeNodePort:
	case corev1.ServiceTypeLoadBalancer:
	default:
		panic("invalid value for service type")
	}
	return svcType
}
