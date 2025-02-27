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
)

const (
	userSecretVolName = "users-config"
	wbSocketsVolName  = "samba-wb-sockets-dir"
	stateVolName      = "samba-state-dir"
	osRunVolName      = "run"
	joinJSONVolName   = "join-data"
)

type volMount struct {
	volume corev1.Volume
	mount  corev1.VolumeMount
}

func getVolumes(vols []volMount) []corev1.Volume {
	v := make([]corev1.Volume, len(vols))
	for i := range vols {
		v[i] = vols[i].volume
	}
	return v
}

func getMounts(vols []volMount) []corev1.VolumeMount {
	m := make([]corev1.VolumeMount, len(vols))
	for i := range vols {
		m[i] = vols[i].mount
	}
	return m
}

func shareVolumeAndMount(planner *sharePlanner, pvcName string) volMount {
	var vmnt volMount
	// volume
	pvcVolName := pvcName + "-smb"
	vmnt.volume = corev1.Volume{
		Name: pvcVolName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
	// mount
	vmnt.mount = corev1.VolumeMount{
		MountPath: planner.sharePath(),
		Name:      pvcVolName,
	}
	return vmnt
}

func configVolumeAndMount(planner *sharePlanner) volMount {
	var vmnt volMount
	// volume
	cmSrc := &corev1.ConfigMapVolumeSource{}
	cmSrc.Name = planner.instanceName()
	vmnt.volume = corev1.Volume{
		Name: configMapName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: cmSrc,
		},
	}
	// mount
	vmnt.mount = corev1.VolumeMount{
		MountPath: planner.containerConfigDir(),
		Name:      configMapName,
	}
	return vmnt
}

func userConfigVolumeAndMount(planner *sharePlanner) volMount {
	var vmnt volMount
	// volume
	uss := planner.userSecuritySource()
	vmnt.volume = corev1.Volume{
		Name: userSecretVolName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: uss.Secret,
				Items: []corev1.KeyToPath{{
					Key:  uss.Key,
					Path: planner.usersConfigFileName(),
				}},
			},
		},
	}
	// mount
	vmnt.mount = corev1.VolumeMount{
		MountPath: planner.usersConfigDir(),
		Name:      userSecretVolName,
	}
	return vmnt
}

func sambaStateVolumeAndMount(planner *sharePlanner) volMount {
	var vmnt volMount
	// todo: should this use a persistent volume?
	// volume
	vmnt.volume = corev1.Volume{
		Name: stateVolName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumDefault,
			},
		},
	}
	// mount
	vmnt.mount = corev1.VolumeMount{
		MountPath: planner.sambaStateDir(),
		Name:      stateVolName,
	}
	return vmnt
}

func osRunVolumeAndMount(planner *sharePlanner) volMount {
	var vmnt volMount
	// volume
	vmnt.volume = corev1.Volume{
		Name: osRunVolName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	// mount
	vmnt.mount = corev1.VolumeMount{
		MountPath: planner.osRunDir(),
		Name:      osRunVolName,
	}
	return vmnt
}

func wbSocketsVolumeAndMount(planner *sharePlanner) volMount {
	var vmnt volMount
	// volume
	vmnt.volume = corev1.Volume{
		Name: wbSocketsVolName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	// mount
	vmnt.mount = corev1.VolumeMount{
		MountPath: planner.winbindSocketsDir(),
		Name:      wbSocketsVolName,
	}
	return vmnt
}

func joinJSONFileVolumeAndMount(planner *sharePlanner, index int) volMount {
	var vmnt volMount
	// volume
	vname := joinJSONVolName + planner.joinJSONSuffix(index)
	j := planner.SecurityConfig.Spec.JoinSources[index]
	vmnt.volume = corev1.Volume{
		Name: vname,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: j.UserJoin.Secret,
				Items: []corev1.KeyToPath{{
					Key:  j.UserJoin.Key,
					Path: planner.joinJSONFileName(),
				}},
			},
		},
	}
	// mount
	vmnt.mount = corev1.VolumeMount{
		MountPath: planner.joinJSONSourceDir(index),
		Name:      vname,
	}
	return vmnt
}

func svcWatchVolumeAndMount(dir string) volMount {
	var vmnt volMount
	// volume
	name := "svcwatch"
	vmnt.volume = corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	// mount
	vmnt.mount = corev1.VolumeMount{
		MountPath: dir,
		Name:      name,
	}
	return vmnt
}

func ctdbConfigVolumeAndMount(_ *sharePlanner) volMount {
	var vmnt volMount
	name := "ctdb-config"
	vmnt.volume = corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	vmnt.mount = corev1.VolumeMount{
		MountPath: "/etc/ctdb",
		Name:      name,
	}
	return vmnt
}

func ctdbPersistentVolumeAndMount(_ *sharePlanner) volMount {
	var vmnt volMount
	// this was an empty dir in my hand-rolled example yaml file
	// but now I'm looking at this and wondering. Keeping it the same
	// for now, but look here first if something seems askance.
	name := "ctdb-persistent"
	vmnt.volume = corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	vmnt.mount = corev1.VolumeMount{
		MountPath: "/var/lib/ctdb/persistent",
		Name:      name,
	}
	return vmnt
}

func ctdbVolatileVolumeAndMount(_ *sharePlanner) volMount {
	var vmnt volMount
	name := "ctdb-volatile"
	vmnt.volume = corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	vmnt.mount = corev1.VolumeMount{
		MountPath: "/var/lib/ctdb/volatile",
		Name:      name,
	}
	return vmnt
}

func ctdbSocketsVolumeAndMount(_ *sharePlanner) volMount {
	var vmnt volMount
	name := "ctdb-sockets"
	vmnt.volume = corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				Medium: corev1.StorageMediumMemory,
			},
		},
	}
	// mount
	vmnt.mount = corev1.VolumeMount{
		MountPath: "/var/run/ctdb",
		Name:      name,
	}
	return vmnt
}

func ctdbSharedStateVolumeAndMount(
	_ *sharePlanner, pvcName string) volMount {
	// ---
	var vmnt volMount
	// we've discussed the possibility of doing without this rwx pvc to
	// bridge the shared state of the ctdb enabled pods, but for now we
	// have not tried any alternatives. so here it is.
	pvcVolName := pvcName + "-ctdb"
	vmnt.volume = corev1.Volume{
		Name: pvcVolName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
			},
		},
	}
	// mount
	vmnt.mount = corev1.VolumeMount{
		MountPath: "/var/lib/ctdb/shared",
		Name:      pvcVolName,
	}
	return vmnt
}
