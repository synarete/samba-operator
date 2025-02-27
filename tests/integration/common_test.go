// +build integration

package integration

import (
	"os"
)

var (
	testNamespace = "samba-operator-system"

	testFilesDir      = "../files"
	operatorConfigDir = "../../config"

	kustomizeCmd = "kustomize"

	testExpectedImage = "quay.io/samba.org/samba-operator:latest"

	testClusteredShares = false
)

func init() {
	ns := os.Getenv("SMBOP_TEST_NAMESPACE")
	if ns != "" {
		testNamespace = ns
	}

	fdir := os.Getenv("SMBOP_TEST_FILES_DIR")
	if fdir != "" {
		testFilesDir = fdir
	}

	cdir := os.Getenv("SMBOP_TEST_CONFIG_DIR")
	if cdir != "" {
		operatorConfigDir = cdir
	}

	km := os.Getenv("SMBOP_TEST_KUSTOMIZE")
	if km != "" {
		kustomizeCmd = km
	}
	km2 := os.Getenv("KUSTOMIZE")
	if km == "" && km2 != "" {
		kustomizeCmd = km2
	}

	timg := os.Getenv("SMBOP_TEST_EXPECT_MANAGER_IMG")
	if timg != "" {
		testExpectedImage = timg
	}

	testClustering := os.Getenv("SMBOP_TEST_CLUSTERED")
	if testClustering != "" {
		testClusteredShares = true
	}
}
