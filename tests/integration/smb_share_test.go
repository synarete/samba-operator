//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	sambaoperatorv1alpha1 "github.com/samba-in-kubernetes/samba-operator/api/v1alpha1"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/dnsclient"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/kube"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/poll"
	"github.com/samba-in-kubernetes/samba-operator/tests/utils/smbclient"
)

type SmbShareSuite struct {
	suite.Suite

	fileSources      []kube.FileSource
	smbShareResource types.NamespacedName
	shareName        string
	testAuths        []smbclient.Auth
	destNamespace    string
	maxPods          int
	minPods          int

	// cached values
	tc *kube.TestClient
}

func (s *SmbShareSuite) SetupSuite() {
	if s.destNamespace == "" {
		s.destNamespace = testNamespace
	}
	if s.maxPods == 0 {
		s.maxPods = 1
	}
	s.tc = kube.NewTestClient("")
	// ensure the smbclient test pod exists
	createSMBClientIfMissing(context.TODO(), s.Require(), s.tc)
	createFromFiles(context.TODO(), s.Require(), s.tc, s.fileSources)
}

func (s *SmbShareSuite) SetupTest() {
	require := s.Require()
	require.NoError(waitForPodExist(context.TODO(), s), "smb server pod does not exist")
	require.NoError(waitForPodReady(context.TODO(), s), "smb server pod is not ready")
}

func (s *SmbShareSuite) TearDownSuite() {
	deleteFromFiles(context.TODO(), s.Require(), s.tc, s.fileSources)
}

func (s *SmbShareSuite) getTestClient() *kube.TestClient {
	return s.tc
}

func (s *SmbShareSuite) getPodFetchOptions() kube.PodFetchOptions {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	return kube.PodFetchOptions{
		Namespace:     s.destNamespace,
		LabelSelector: l,
		MaxFound:      s.maxPods,
		MinFound:      s.minPods,
	}
}

func (s *SmbShareSuite) getPodIP() (string, error) {
	pod, err := s.getReadyPod()
	if err != nil {
		return "", err
	}
	return pod.Status.PodIP, nil
}

func (s *SmbShareSuite) getReadyPod() (*corev1.Pod, error) {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	pods, err := s.tc.FetchPods(
		context.TODO(),
		kube.PodFetchOptions{
			Namespace:     s.destNamespace,
			LabelSelector: l,
			MaxFound:      s.maxPods,
		})
	if err != nil {
		return nil, err
	}
	for _, pod := range pods {
		if kube.PodIsReady(&pod) {
			return &pod, nil
		}
	}
	return nil, fmt.Errorf("no pods ready")
}

func (s *SmbShareSuite) TestPodsReady() {
	s.Require().NoError(waitForPodReady(context.TODO(), s))
}

func (s *SmbShareSuite) TestSmbShareServerGroup() {
	smbShare := &sambaoperatorv1alpha1.SmbShare{}
	err := s.tc.TypedObjectClient().Get(
		context.TODO(), s.smbShareResource, smbShare)
	s.Require().NoError(err)
	s.Require().Equal(s.smbShareResource.Name, smbShare.Name)
	s.Require().Equal(s.smbShareResource.Name, smbShare.Status.ServerGroup)
}

func (s *SmbShareSuite) TestShareAccessByIP() {
	ip, err := s.getPodIP()
	s.Require().NoError(err)
	shareAccessSuite := &ShareAccessSuite{
		share: smbclient.Share{
			Host: smbclient.Host(ip),
			Name: s.shareName,
		},
		auths: s.testAuths,
	}
	suite.Run(s.T(), shareAccessSuite)
}

func (s *SmbShareSuite) TestShareAccessByServiceName() {
	svcname := fmt.Sprintf("%s.%s.svc.cluster.local",
		s.smbShareResource.Name,
		s.destNamespace)
	shareAccessSuite := &ShareAccessSuite{
		share: smbclient.Share{
			Host: smbclient.Host(svcname),
			Name: s.shareName,
		},
		auths: s.testAuths,
	}
	suite.Run(s.T(), shareAccessSuite)
}

func (s *SmbShareSuite) TestShareEvents() {
	s.Require().NoError(waitForPodReady(context.TODO(), s))

	// this unstructured stuff is just to get a UID for the SmbShare for event
	// filtering. Since the tests don't currently have a way to use a typed
	// interface for API access to SmbShare we take the lazy way out
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("samba-operator.samba.org/v1alpha1")
	u.SetKind("SmbShare")
	dc, err := s.tc.DynamicClientset(u)
	s.Require().NoError(err)
	u, err = dc.Namespace(s.smbShareResource.Namespace).Get(
		context.TODO(),
		s.smbShareResource.Name,
		metav1.GetOptions{})
	s.Require().NoError(err)

	l, err := s.tc.Clientset().CoreV1().Events(s.smbShareResource.Namespace).List(
		context.TODO(),
		metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.kind=SmbShare,involvedObject.name=%s,involvedObject.uid=%s", s.smbShareResource.Name, u.GetUID()),
		})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(l.Items), 1)
	numCreatedPVC := 0
	numCreatedInstance := 0
	for _, event := range l.Items {
		if event.Reason == "CreatedPersistentVolumeClaim" {
			numCreatedPVC++
		}
		if event.Reason == "CreatedDeployment" {
			numCreatedInstance++
		}
		if event.Reason == "CreatedStatefulSet" {
			numCreatedInstance++
		}
	}
	s.Require().Equal(1, numCreatedPVC)
	s.Require().Equal(1, numCreatedInstance)
}

type SmbShareWithDNSSuite struct {
	SmbShareSuite
}

func (s *SmbShareWithDNSSuite) TestShareAccessByDomainName() {
	dnsname := fmt.Sprintf("%s-cluster.domain1.sink.test",
		s.smbShareResource.Name)
	lbl := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.smbShareResource.Name)

	// get the ip of the service that should have been added to the ad dns. We
	// take these extra steps to help disentangle dns update, caching, or
	// resolution problems from access to the share when using DNS names.
	// Previously, we just had a sleep but it was an unreliable workaround and
	// it didn't help detect where the problem was.
	sl, err := s.tc.Clientset().CoreV1().Services(s.destNamespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: lbl,
		},
	)
	s.Require().NoError(err, "failed to get services")
	s.Require().Len(sl.Items, 1, "expected exactly one matching service")
	svcClusterIP := sl.Items[0].Spec.ClusterIP
	// test that the IP in ad dns matches the service
	ctx, cancel := context.WithDeadline(
		context.TODO(),
		time.Now().Add(waitForIpTime))
	defer cancel()
	hc := dnsclient.MustPodExec(s.tc, testNamespace, "smbclient", "")
	s.Require().NoError(poll.TryUntil(ctx, &poll.Prober{
		Cond: func() (bool, error) {
			ip4addr, err := hc.HostAddress(dnsname)
			if err != nil {
				return false, nil
			}
			return ip4addr == svcClusterIP, nil
		},
	}))

	shareAccessSuite := &ShareAccessSuite{
		share: smbclient.Share{
			Host: smbclient.Host(dnsname),
			Name: s.shareName,
		},
		auths: s.testAuths,
	}
	suite.Run(s.T(), shareAccessSuite)
}

func (s *SmbShareWithDNSSuite) TestPodForDNSContainers() {
	l := fmt.Sprintf(
		"samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	pods, err := s.tc.FetchPods(
		context.TODO(),
		kube.PodFetchOptions{
			Namespace:     s.destNamespace,
			LabelSelector: l,
			MaxFound:      s.maxPods,
		},
	)
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(pods[0].Spec.Containers), 4)
	names := []string{}
	for _, cstatus := range pods[0].Status.ContainerStatuses {
		names = append(names, cstatus.Name)
		s.Require().True(cstatus.Ready, "container %s not ready", cstatus.Name)
	}
	s.Require().Contains(names, "dns-register")
	s.Require().Contains(names, "svc-watch")
}

type SmbShareWithExternalNetSuite struct {
	SmbShareSuite
}

func (s *SmbShareWithExternalNetSuite) TestServiceIsLoadBalancer() {
	lbl := fmt.Sprintf("samba-operator.samba.org/service=%s", s.smbShareResource.Name)
	l, err := s.tc.Clientset().CoreV1().Services(s.destNamespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector: lbl,
		},
	)
	s.Require().NoError(err)
	s.Require().Len(l.Items, 1)
	// our test environment does not require the k8s cluster to actually
	// support an external load balancer. All this test can do is check
	// IF LoadBalanacer was set.
	svc := l.Items[0]
	s.Require().Equal(
		corev1.ServiceTypeLoadBalancer,
		svc.Spec.Type,
	)
}

func (s *SmbShareSuite) TestMetricsOnPod() {
	s.Require().NoError(waitForPodReady(context.TODO(), s))

	pod, cont, err := s.getMetricsContainer()
	s.Require().NoError(err)
	if cont == nil {
		s.T().Skipf("no metrics container present")
	}
	// Issue a curl command from samba container to samba-metrics container
	// within smbd pod.
	curl := fmt.Sprintf("curl -s http://%s:8080/metrics", pod.GetName())
	pc := kube.PodCommand{
		Command:       []string{"sh", "-c", curl},
		Namespace:     pod.GetNamespace(),
		PodName:       pod.GetName(),
		ContainerName: "samba",
	}
	bch := kube.NewBufferedCommandHandler()
	err = kube.NewTestExec(s.tc).Call(pc, bch)
	s.Require().NoError(err)
	out := strings.TrimSpace(string(bch.GetStdout()))
	s.Require().NotEmpty(out)

	// Ensure that we get at least minimal output
	hasLocksTotal := false
	hasSharesTotal := false
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "smb_locks_total ") {
				hasLocksTotal = true
			} else if strings.HasPrefix(line, "smb_shares_total ") {
				hasSharesTotal = true
			}
		}
	}
	s.Require().True(hasLocksTotal)
	s.Require().True(hasSharesTotal)
}

func (s *SmbShareSuite) getMetricsContainer() (
	*corev1.Pod, *corev1.Container, error) {
	pod, err := s.getReadyPod()
	if err != nil {
		return nil, nil, err
	}
	for _, cont := range pod.Spec.Containers {
		if strings.Contains(cont.Name, "metrics") {
			return pod, &cont, nil
		}
	}
	return nil, nil, nil // Case running without metrics
}

func init() {
	utilruntime.Must(sambaoperatorv1alpha1.AddToScheme(kube.TypedClientScheme))

	smbShareTests := testRoot.ChildPriority("smbShares", 1)
	smbShareTests.AddSuite("users1", &SmbShareSuite{
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbshare1.yaml"),
				Namespace: testNamespace,
			},
		},
		smbShareResource: types.NamespacedName{testNamespace, "tshare1"},
		shareName:        "My Share",
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
	},
	)

	smbShareTests.AddSuite("domainMember1", &SmbShareWithDNSSuite{SmbShareSuite{
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "joinsecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig2.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbshare2.yaml"),
				Namespace: testNamespace,
			},
		},
		smbShareResource: types.NamespacedName{testNamespace, "tshare2"},
		shareName:        "My Kingdom",
		testAuths: []smbclient.Auth{{
			Username: "DOMAIN1\\bwayne",
			Password: "1115Rose.",
		}},
	}},
	)

	// Test that the operator functions when the SmbShare resources are created
	// in a different ns (for example, "default").
	// IMPORTANT: the secrets MUST be in the same namespace as the pods.
	smbShareTests.AddSuite("smbSharesInDefault", &SmbShareSuite{
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: "default",
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: "default",
			},
			{
				Path:      path.Join(testFilesDir, "smbshare3.yaml"),
				Namespace: "default",
			},
		},
		smbShareResource: types.NamespacedName{"default", "tshare3"},
		destNamespace:    "default",
		shareName:        "My Other Share",
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
	},
	)

	smbShareTests.AddSuite("smbSharesExternal", &SmbShareWithExternalNetSuite{SmbShareSuite{
		fileSources: []kube.FileSource{
			{
				Path:      path.Join(testFilesDir, "userssecret1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "commonconfig1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
				Namespace: testNamespace,
			},
			{
				Path:      path.Join(testFilesDir, "smbshare4.yaml"),
				Namespace: testNamespace,
			},
		},
		smbShareResource: types.NamespacedName{testNamespace, "tshare4"},
		shareName:        "Since When",
		testAuths: []smbclient.Auth{{
			Username: "sambauser",
			Password: "1nsecurely",
		}},
	}},
	)

	if testClusteredShares {
		clusteredTests := testRoot.ChildPriority("smbSharesClustered", 1)
		clusteredTests.AddSuite("default", &SmbShareSuite{
			fileSources: []kube.FileSource{
				{
					Path:      path.Join(testFilesDir, "userssecret1.yaml"),
					Namespace: testNamespace,
				},
				{
					Path:      path.Join(testFilesDir, "smbsecurityconfig1.yaml"),
					Namespace: testNamespace,
				},
				{
					Path:      path.Join(testFilesDir, "smbshare_ctdb1.yaml"),
					Namespace: testNamespace,
				},
			},
			smbShareResource: types.NamespacedName{testNamespace, "cshare1"},
			maxPods:          3,
			shareName:        "CTDB Me",
			testAuths: []smbclient.Auth{{
				Username: "bob",
				Password: "r0b0t",
			}},
		},
		)

		clusteredTests.AddSuite("noDNS", &SmbShareSuite{
			fileSources: []kube.FileSource{
				{
					Path:      path.Join(testFilesDir, "joinsecret1.yaml"),
					Namespace: testNamespace,
				},
				{
					Path:      path.Join(testFilesDir, "smbsecurityconfig2.yaml"),
					Namespace: testNamespace,
				},
				{
					Path:       path.Join(testFilesDir, "smbshare_ctdb2.yaml"),
					Namespace:  testNamespace,
					NameSuffix: "-dmo",
				},
			},
			smbShareResource: types.NamespacedName{testNamespace, "cshare2-dmo"},
			maxPods:          3,
			shareName:        "Three Kingdoms",
			testAuths: []smbclient.Auth{{
				Username: "DOMAIN1\\ckent",
				Password: "1115Rose.",
			}},
		},
		)

		clusteredTests.AddSuite("withDNS", &SmbShareWithDNSSuite{SmbShareSuite{
			fileSources: []kube.FileSource{
				{
					Path:      path.Join(testFilesDir, "joinsecret1.yaml"),
					Namespace: testNamespace,
				},
				{
					Path:      path.Join(testFilesDir, "smbsecurityconfig2.yaml"),
					Namespace: testNamespace,
				},
				{
					Path:       path.Join(testFilesDir, "smbshare_ctdb2.yaml"),
					Namespace:  testNamespace,
					NameSuffix: "-dmdns",
				},
			},
			smbShareResource: types.NamespacedName{testNamespace, "cshare2-dmdns"},
			maxPods:          3,
			minPods:          2,
			shareName:        "Three Kingdoms",
			testAuths: []smbclient.Auth{{
				Username: "DOMAIN1\\ckent",
				Password: "1115Rose.",
			}},
		}},
		)

		clusteredTests.AddSuite("external", &SmbShareWithExternalNetSuite{SmbShareSuite{
			fileSources: []kube.FileSource{
				{
					Path:      path.Join(testFilesDir, "joinsecret1.yaml"),
					Namespace: testNamespace,
				},
				{
					Path:      path.Join(testFilesDir, "commonconfig1.yaml"),
					Namespace: testNamespace,
				},
				{
					Path:      path.Join(testFilesDir, "smbsecurityconfig2.yaml"),
					Namespace: testNamespace,
				},
				{
					Path:       path.Join(testFilesDir, "smbshare_ctdb3.yaml"),
					Namespace:  testNamespace,
					NameSuffix: "-exlb",
				},
			},
			smbShareResource: types.NamespacedName{testNamespace, "cshare3-exlb"},
			maxPods:          3,
			minPods:          2,
			shareName:        "Costly Hare",
			testAuths: []smbclient.Auth{{
				Username: "DOMAIN1\\bwayne",
				Password: "1115Rose.",
			}},
		}},
		)
	}
}
