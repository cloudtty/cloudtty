package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cloudshellv1alpha1 "github.com/cloudtty/cloudtty/pkg/apis/cloudshell/v1alpha1"
)

const (
	operatorImage      = "ghcr.io/cloudtty/cloudshell-operator:e2e"
	cloudshellImage    = "ghcr.io/cloudtty/cloudshell:e2e"
	operatorNamespace  = "cloudtty-system"
	operatorDeployment = "cloudtty-controller-manager"
)

var (
	cfg       *rest.Config
	k8sClient client.Client
)

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func isKindClusterRunning() bool {
	cmd := exec.Command("kind", "get", "clusters")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "kind")
}

func getProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

func imageExistsLocally(image string) bool {
	cmd := exec.Command("docker", "images", "-q", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return len(bytes.TrimSpace(output)) > 0
}

func buildDockerImage(image, dockerfile string) {
	if imageExistsLocally(image) {
		fmt.Fprintf(GinkgoWriter, "Image %s already exists locally, skipping build.\n", image)
		return
	}
	projectRoot := getProjectRoot()
	cmd := exec.Command("docker", "build", "-t", image, projectRoot, "-f", filepath.Join(projectRoot, dockerfile))
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func loadImageToKind(image string) {
	cmd := exec.Command("kind", "load", "docker-image", image, "--name", "kind")
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func installOperator() {
	projectRoot := getProjectRoot()
	cmd := exec.Command("helm", "upgrade", "--install", "cloudtty", "./charts/cloudtty",
		"--namespace", operatorNamespace,
		"--create-namespace",
		"--set", "image.tag=e2e",
		"--set", "cloudshell.image.tag=e2e",
		"--set", "image.pullPolicy=IfNotPresent",
		"--set", "cloudshell.image.pullPolicy=IfNotPresent",
		"--set", "coreWorkerLimit=2",
		"--set", "maxWorkerLimit=5",
		"--wait",
		"--timeout", "3m",
	)
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
}

func uninstallOperator() {
	cmd := exec.Command("helm", "uninstall", "cloudtty", "--namespace", operatorNamespace, "--wait", "--timeout", "2m")
	output, err := cmd.CombinedOutput()
	// Allow not-found errors during cleanup
	if err != nil && !strings.Contains(string(output), "not found") {
		fmt.Fprintf(GinkgoWriter, "helm uninstall warning: %s\n", string(output))
	}
}

func initK8sClient() {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	Expect(err).NotTo(HaveOccurred())
	cfg = config

	err = cloudshellv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
}

func isDeploymentReady(namespace, name string) bool {
	dep := &appsv1.Deployment{}
	err := k8sClient.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, dep)
	if err != nil {
		return false
	}
	if dep.Status.Replicas == 0 {
		return false
	}
	return dep.Status.ReadyReplicas == dep.Status.Replicas
}

func isPodRunning(namespace string, labels map[string]string) bool {
	podList := &corev1.PodList{}
	err := k8sClient.List(context.Background(), podList, client.InNamespace(namespace), client.MatchingLabels(labels))
	if err != nil || len(podList.Items) == 0 {
		return false
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			return false
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status != corev1.ConditionTrue {
				return false
			}
		}
	}
	return true
}

func getPodLogs(namespace, podName, containerName string) string {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Sprintf("failed to create clientset: %v", err)
	}

	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: containerName,
	})
	readCloser, err := req.Stream(context.Background())
	if err != nil {
		return fmt.Sprintf("failed to get log stream: %v", err)
	}
	defer readCloser.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, readCloser)
	if err != nil {
		return fmt.Sprintf("failed to read logs: %v", err)
	}
	return buf.String()
}

func waitForPortForward(localPort int) bool {
	for i := 0; i < 30; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), 1*time.Second)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// portForwardService starts a kubectl port-forward for a service and returns the local port.
// The caller is responsible for killing the process when done.
func portForwardService(namespace, serviceName string, remotePort int) (*exec.Cmd, int, error) {
	localPort := 18080 + remotePort // arbitrary local port to avoid conflicts
	cmd := exec.Command("kubectl", "-n", namespace, "port-forward",
		fmt.Sprintf("service/%s", serviceName),
		fmt.Sprintf("%d:%d", localPort, remotePort),
	)
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	// Wait for port-forward to be ready
	if !waitForPortForward(localPort) {
		_ = cmd.Process.Kill()
		return nil, 0, fmt.Errorf("port-forward did not become ready")
	}
	return cmd, localPort, nil
}

// portForwardPod starts a kubectl port-forward for a pod and returns the local port.
// The caller is responsible for killing the process when done.
func portForwardPod(namespace, podName string, remotePort int) (*exec.Cmd, int, error) {
	localPort := 19080 + remotePort // arbitrary local port to avoid conflicts
	cmd := exec.Command("kubectl", "-n", namespace, "port-forward",
		fmt.Sprintf("pod/%s", podName),
		fmt.Sprintf("%d:%d", localPort, remotePort),
	)
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	// Wait for port-forward to be ready
	if !waitForPortForward(localPort) {
		_ = cmd.Process.Kill()
		return nil, 0, fmt.Errorf("port-forward did not become ready")
	}
	return cmd, localPort, nil
}

func httpGet(url string) (int, string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body := new(bytes.Buffer)
	_, _ = io.Copy(body, resp.Body)
	return resp.StatusCode, body.String(), nil
}

func deleteNamespaceIgnoreNotFound(ctx context.Context, name string) {
	ns := &corev1.Namespace{}
	ns.Name = name
	err := k8sClient.Delete(ctx, ns)
	if err != nil && !apierrors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred())
	}
}
