package sshproxy

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"

	"github.com/cloudtty/cloudtty/pkg/sshproxy/clients"
)

const DefaultPodProxyServerPort = 2223

func PodProxyServer(ctx context.Context, port int) {
	if port == 0 {
		port = DefaultPodProxyServerPort
	}
	serverConfig, err := newSSHServerConfig()
	if err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("ssh proxy pod server listen %d port \n", port)
	for {
		select {
		case <-ctx.Done():
			klog.Infof("PodProxyServer exit")
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("accept tcp[:%d] err: %v", port, err)
				continue
			}
			go HandleConnWithContext(ctx, conn, serverConfig)
		}
	}
}

func newSSHServerConfig() (*ssh.ServerConfig, error) {
	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	client := clients.GetClients().GetGlobalKubeClient()
	namespace := clients.GetClients().GetOptions().HostKeySecretNamespace
	secretName := clients.GetClients().GetOptions().HostKeySecretName
	secret, err := client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, v12.GetOptions{})
	if err != nil {
		return nil, err
	}
	keyData, ok := secret.Data["ssh-privatekey"]
	if !ok {
		return nil, fmt.Errorf("ssh-privatekey is nil")
	}
	privateSigner, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, err
	}
	config.AddHostKey(privateSigner)
	return config, nil
}

func handleSession(ctx context.Context, c ssh.Channel, rc <-chan *ssh.Request, user string) error {
	defer func() {
		err := c.Close()
		if err != nil {
			klog.Errorf("failed to close channel, err: %v", err)
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case r, ok := <-rc:
				if !ok {
					return
				}
				switch r.Type {
				case "pty-req":
					err := r.Reply(true, nil)
					if err != nil {
						klog.Errorf("err: %v", err)
					}
				default:
					klog.Infof("r.Type: %s", r.Type)
				}
			}
		}
	}()
	userName, clusterName, namespace, podName, containerName, err := checkUserForPod(user)
	if err != nil {
		writeErrToChan(c, fmt.Errorf("check user failed: %w", err))
		return err
	}
	log.Printf("userName: %s, cluster: %s, pod: %s, namespace: %s, container: %s \n", userName, clusterName, podName, namespace, containerName)
	cloudshellClient := clients.GetClients().GetCloudshellClient()
	cloudProxy, err := cloudshellClient.CloudshellV1alpha1().CloudProxies().Get(context.TODO(), clusterName, v12.GetOptions{})
	if err != nil {
		writeErrToChan(c, fmt.Errorf("failed to get CloudProxy for cluster %s: %w", clusterName, err))
		return err
	}
	config, err := clients.GetClientConfigByProxy(cloudProxy)
	if err != nil {
		writeErrToChan(c, fmt.Errorf("failed to get client config for proxy %s: %w", cloudProxy.Name, err))
		return err
	}
	client, err := clientset.NewForConfig(config)
	if err != nil {
		writeErrToChan(c, fmt.Errorf("failed to creat Kubernetes client for proxy %s: %w", cloudProxy.Name, err))
		return err
	}
	config.Impersonate.UserName = userName
	// TODO: support sh and bash
	return execPod(ctx, client, config, c, userName, podName, namespace, containerName, []string{"sh"})
}

func writeErrToChan(c ssh.Channel, err error) {
	_, e := c.Write([]byte(err.Error() + ". "))
	if e != nil {
		log.Printf("write err [%s] to chan failed, err: %v", err.Error(), e)
	}
}

func HandleConnWithContext(ctx context.Context, conn net.Conn, serverConfig *ssh.ServerConfig) {
	serverConn, chans, reqs, err := ssh.NewServerConn(conn, serverConfig)
	logger := logrus.WithFields(logrus.Fields{"user": serverConn.User(), "remoteAddr": conn.RemoteAddr(), "sessionID": serverConn.SessionID()})
	if err != nil {
		logger.Errorf("ssh NewServerConn err: %v, remote addr: %s, local addr: %s", err, conn.RemoteAddr(), conn.LocalAddr())
		return
	}
	go ssh.DiscardRequests(reqs)
	// channel type contains: 'session', 'x11', 'forwarded-tcpip', 'direct-tcpip', details: https://www.rfc-editor.org/rfc/rfc4250#section-4.9.1
	for {
		select {
		case <-ctx.Done():
			return
		default:
			c, ok := <-chans
			if !ok {
				return
			}
			switch c.ChannelType() {
			case "session":
				c, r, err := c.Accept()
				if err != nil {
					logger.Errorf("accept session channel error: %v", err)
				}
				go handleSession(ctx, c, r, serverConn.User())
			default:
				err = c.Reject(ssh.UnknownChannelType, "unsupported channel type")
				if err != nil {
					logger.Errorf("ssh Reject chanel[%s] err: %v, remote addr: %s, local addr: %s", err, c.ChannelType(), conn.RemoteAddr(), conn.LocalAddr())
				}
			}
		}
	}
}

func execPod(ctx context.Context, clientset clientset.Interface, config *rest.Config, session ssh.Channel, userName, podName, namespace, containerName string, command []string) error {
	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
			Container: containerName,
			Command:   command,
		}, scheme.ParameterCodec)
	url := req.URL()
	executor, err := remotecommand.NewSPDYExecutor(config, "POST", url)
	if err != nil {
		return fmt.Errorf("Error executing command, pod: %s, namespace: %s, containerName: %s, command: %v, err: %v\n", podName, namespace, containerName, command, err)
	}

	err = executor.StreamWithContext(ctx,
		remotecommand.StreamOptions{
			Stdin:  session,
			Stdout: session,
			Stderr: session,
			Tty:    true,
		},
	)
	if err != nil {
		writeErrToChan(session, err)
		return fmt.Errorf("Error executing command, pod: %s, namespace: %s, containerName: %s, command: %v, err: %v\n", podName, namespace, containerName, command, err)
	}
	return nil
}

// TODO: if container not specified, set default container
// if login pod, the user info should like admin@cluster.ns.pod or admin@cluster.ns.pod.container.
func checkUserForPod(user string) (userName, cluster, namespace, podName, containerName string, err error) {
	userArr := strings.Split(user, "@")
	if len(userArr) < 2 {
		err = fmt.Errorf("useInfo[%s] format err", user)
		klog.Error(err)
		return
	}
	userName = userArr[0]
	target := userArr[1]
	targetPodInfo := strings.Split(target, ".")
	length := len(targetPodInfo)
	if length < 3 || length > 4 {
		err = fmt.Errorf("target podInfo[%s] format err", target)
		klog.Error(err)
		return
	}
	if length == 4 {
		containerName = targetPodInfo[3]
	}
	cluster, namespace, podName = targetPodInfo[0], targetPodInfo[1], targetPodInfo[2]
	return
}
