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

package smbcc

// Key values are used to select subsections in the container config.
type Key string

// FeatureFlag values are used to select top level features that
// sambacc will apply when setting up a container.
type FeatureFlag string

const (
	// CTDB feature flag indicates the system should be configured with CTDB.
	CTDB FeatureFlag = "ctdb"
)

// SambaContainerConfig holds one or more configuration for samba
// containers.
type SambaContainerConfig struct {
	SCCVersion string                `json:"samba-container-config"`
	Configs    map[Key]ConfigSection `json:"configs,omitempty"`
	Shares     map[Key]ShareConfig   `json:"shares,omitempty"`
	Globals    map[Key]GlobalConfig  `json:"globals,omitempty"`
	Users      map[Key]UserEntries   `json:"users,omitempty"`
	Groups     map[Key]GroupEntries  `json:"groups,omitempty"`
}

// ConfigSection identifies the shares, globals, and instance name of
// a single configuration.
type ConfigSection struct {
	Shares           []Key         `json:"shares,omitempty"`
	Globals          []Key         `json:"globals,omitempty"`
	InstanceName     string        `json:"instance_name,omitempty"`
	InstanceFeatures []FeatureFlag `json:"instance_features,omitempty"`
}

// ShareConfig holds configuration values for one share.
type ShareConfig struct {
	Options SmbOptions `json:"options,omitempty"`
}

// GlobalConfig holds configuration values for samba server globals.
type GlobalConfig struct {
	Options SmbOptions `json:"options,omitempty"`
}

// UserEntry represents a single "local" user for share access.
type UserEntry struct {
	Name     string `json:"name"`
	Uid      uint   `json:"uid,omitempty"`
	Gid      uint   `json:"gid,omitempty"`
	NTHash   string `json:"nt_hash,omitempty"`
	Password string `json:"password,omitempty"`
}

// UserEntries is a slice of UserEntry values.
type UserEntries []UserEntry

// GroupEntry represents a single "local" group for share access.
type GroupEntry struct {
	Name string `json:"name"`
	Gid  uint   `json:"gid,omitempty"`
}

// GroupEntries is a slice of GroupEntry values.
type GroupEntries []GroupEntry

// SmbOptions is a common type for storing smb.conf parameters.
type SmbOptions map[string]string

const version0 = "v0"

const (
	// NoPrintingKey is used for the standard "noprinting" globals subsection.
	NoPrintingKey = Key("noprinting")
	// AllEntriesKey is used for the standard "all_entries" default key for
	// users and groups.
	AllEntriesKey = Key("all_entries")

	// BrowseableParam controls if a share is browseable.
	BrowseableParam = "browseable"
	// ReadOnlyParam controls if a share is read only.
	ReadOnlyParam = "read only"

	// Yes means yes.
	Yes = "yes"
	// No means no.
	No = "no"
)

// New returns a new samba container config.
func New() *SambaContainerConfig {
	return &SambaContainerConfig{
		SCCVersion: version0,
		Configs:    map[Key]ConfigSection{},
		Shares:     map[Key]ShareConfig{},
		Globals:    map[Key]GlobalConfig{},
	}
}

// NewNoPrintingGlobals returns a GlobalConfig that disables printing.
func NewNoPrintingGlobals() GlobalConfig {
	return GlobalConfig{
		Options: SmbOptions{
			"load printers":   No,
			"printing":        "bsd",
			"printcap name":   "/dev/null",
			"disable spoolss": Yes,
		},
	}
}

// NewSimpleShare returns a ShareConfig with a simple configuration.
func NewSimpleShare(path string) ShareConfig {
	return ShareConfig{
		Options: SmbOptions{
			"path":      path,
			"read only": No,
		},
	}
}

// NewConfigSection returns a new ConfigSection.
func NewConfigSection(name string) ConfigSection {
	return ConfigSection{
		Shares:       []Key{},
		Globals:      []Key{},
		InstanceName: name,
	}
}

// NewDefaultUsers returns a full subsection for a default (good for testing)
// set of users.
func NewDefaultUsers() map[Key]UserEntries {
	return map[Key]UserEntries{
		AllEntriesKey: {{
			Name:     "sambauser",
			Password: "samba",
		}},
	}
}
