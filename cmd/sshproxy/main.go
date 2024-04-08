package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudtty/cloudtty/cmd/sshproxy/options"
	"github.com/cloudtty/cloudtty/pkg/sshproxy"
	"github.com/cloudtty/cloudtty/pkg/sshproxy/clients"
	"github.com/sirupsen/logrus"
	"github.com/tg123/sshpiper/libplugin"
	"github.com/urfave/cli/v2"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfig "k8s.io/component-base/config"
	"k8s.io/klog/v2"
	"k8s.io/utils/net"
)

type UserLoginType int32

// ssh root@cluster1.node1@10.6.111.111
// ssh root@10.5.2.1@10.6.111.111
// ssh cluster1.ns1.app1@10.6.111.111

const (
	INVALID UserLoginType = iota
	IPAddress
	AccessName
)

var (
	arg1 string
	port int
)

func main() {
	createPlugin()
}

func createPlugin() {
	libplugin.CreateAndRunPluginTemplate(&libplugin.PluginTemplate{
		Name:  "ssh-proxy",
		Usage: "Proxy for login nodes in kubernetes cluster",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "kubeconfig",
				Usage:    "kubeconfig for cluster.",
				Required: false,
			},
			&cli.IntFlag{
				Name:     "port",
				Usage:    "Pod proxy server port number.",
				Required: false,
				Value:    2223,
			},
			&cli.StringFlag{
				Name:     "hostkey-secret-namespace",
				Usage:    "Where the ssh private key and cloudproxy created.",
				Required: true,
				Value:    "",
			},
			&cli.StringFlag{
				Name:     "hostkey-secret-name",
				Usage:    "Secret for ssh host key.",
				Required: true,
				Value:    "",
			},
		},
		CreateConfig: func(c *cli.Context) (*libplugin.SshPiperPluginConfig, error) {
			opt := &options.Options{
				ClientConnection:       componentbaseconfig.ClientConnectionConfiguration{},
				Master:                 "",
				Kubeconfig:             c.String("kubeconfig"),
				HostKeySecretNamespace: c.String("hostkey-secret-namespace"),
				HostKeySecretName:      c.String("hostkey-secret-name"),
			}
			klog.Infof("options: %v", opt)
			err := clients.InitClient(opt)
			if err != nil {
				klog.Fatalf("failed to init client, err: %v", err)
			}
			go sshproxy.PodProxyServer(c.Context, port)
			return &libplugin.SshPiperPluginConfig{
				PasswordCallback: func(conn libplugin.ConnMetadata, password []byte) (*libplugin.Upstream, error) {
					logger := logrus.WithFields(logrus.Fields{"user": conn.User(), "remoteAddr": conn.RemoteAddr(), "uniqueID": conn.UniqueID()})
					logger.Infof("received connection")
					usertype, username, target, err := checkUserForNode(conn.User())
					if err != nil {
						logger.Errorf("check user failed, err: %v", err)
						return nil, err
					}
					ip := target
					if usertype == AccessName {
						ip, err = parseNodeIP(target)
						if err != nil {
							logger.Errorf("parseNodeIP failed, err: %v", err)
							return nil, err
						}
					}
					logger.Infof("username: %s, node ip: %s", username, ip)
					return &libplugin.Upstream{
						Host:          ip,
						Port:          22,
						UserName:      username,
						IgnoreHostKey: true,
						Auth:          libplugin.CreatePasswordAuth(password),
					}, nil
				},
			}, nil
		},
	})
}

// target node like cluster1.node1
func parseNodeIP(target string) (string, error) {
	nodeInfo := strings.Split(target, ".")
	if len(nodeInfo) != 2 {
		return "", fmt.Errorf("node info err: %s", nodeInfo)
	}
	proxyName, nodeName := nodeInfo[0], nodeInfo[1]
	cloudshellClient := clients.GetClients().GetCloudshellClient()
	cloudProxy, err := cloudshellClient.CloudshellV1alpha1().CloudProxies().Get(context.TODO(), proxyName, v1.GetOptions{})
	if err != nil {
		klog.Errorf("get CloudProxies err: %v, name: %s", err, proxyName)
		return "", err
	}
	clientset, err := clients.GetClientsetByProxy(cloudProxy)
	if err != nil {
		return "", err
	}
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, v1.GetOptions{})
	if err != nil {
		return "", err
	}
	if len(node.Status.Addresses) == 0 {
		klog.Errorf("node ip is empty, node: %s", node.Name)
		return "", nil
	}
	return node.Status.Addresses[0].Address, nil
}

// if login node, the user info should like root@cluster.node
func checkUserForNode(user string) (UserLoginType, string, string, error) {
	userArr := strings.Split(user, "@")
	if len(userArr) < 2 {
		return INVALID, "", "", fmt.Errorf("user info format not correct: %s", user)
	}
	username, target := userArr[0], userArr[1]
	if net.IsIPv4String(target) {
		return IPAddress, username, target, nil
	}
	nodeInfos := strings.Split(target, ".")
	if len(nodeInfos) != 2 {
		return INVALID, "", "", fmt.Errorf("node info format not correct: %s", nodeInfos)
	}
	return AccessName, username, target, nil
}
