/*
Copyright 2022.

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

package manifests

const (
	JobTmplV1 = `
  apiVersion: batch/v1
  kind: Job
  metadata:
    namespace: {{ .Namespace }}
    name: {{ .Name }}
    labels:
      ownership: {{ .Ownership }}
  spec:
    activeDeadlineSeconds: 3600
    ttlSecondsAfterFinished: 60
    parallelism: 1
    completions: 1
    template:
      spec:
        automountServiceAccountToken: false
        containers:
        - name:  web-tty
          image: ghcr.io/cloudtty/cloudshell:v0.3.0
          imagePullPolicy: IfNotPresent
          ports:
          - containerPort: 7681
            name: tty-ui
            protocol: TCP
          command:
            - bash
            - "-c"
            - |
              once=""
              index=""
              if [ "${ONCE}" == "true" ];then once=" --once "; fi;
              if [ -f /index.html ]; then index=" --index /index.html ";fi
              if [ "${URLARG}" == "true" ];then urlarg=" -a "; fi
              if [ -z "${TTL}" ] || [ "${TTL}" == "0" ];then
                  ttyd ${index} ${once} ${urlarg} sh -c "${COMMAND}"
              else
                  timeout ${TTL} ttyd ${index} ${once} ${urlarg} sh -c "${COMMAND}" || echo "exiting"
              fi
          env:
          - name: KUBECONFIG
            value: /usr/local/kubeconfig/config
          - name: ONCE
            value: "{{ .Once }}"
          - name: TTL
            value: "{{ .Ttl }}"
          - name: COMMAND
            value: {{ .Command }}
          - name: URLARG
            value: "{{ .UrlArg }}"
          volumeMounts:
            - mountPath: /usr/local/kubeconfig/
              name: kubeconfig
          readinessProbe:
            tcpSocket:
              port: 7681
            periodSeconds: 1
            failureThreshold: 15
          livenessProbe:
            tcpSocket:
              port: 7681
            periodSeconds: 20
        restartPolicy: Never
        volumes:
        - configMap:
            defaultMode: 420
            name: {{ .Configmap }}
          name: kubeconfig
`

	ServiceTmplV1 = `
apiVersion: v1
kind: Service
metadata:
  labels:
    ownership: {{ .Ownership }}
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  ports:
  - name: ttyd
    port: 7681
    protocol: TCP
    targetPort: 7681
  selector:
    job-name: {{ .JobName }}
  type: {{ .Type }}
`

	IngressTmplV1 = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: {{ .IngressClassName }}
  rules:
  - http:
      paths:
      - path: {{ .Path }}
        pathType: Prefix
        backend:
          service:
            name: {{ .ServiceName }}
            port:
              number: 7681
`

	VirtualServiceV1Beta1 = `
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  exportTo:
  - {{ .ExportTo }}
  gateways:
  - {{ .Gateway }}
  hosts:
  - '*'
  http:
  - match:
    - uri:
        prefix: {{ .Path }}
    rewrite:
      uri: /
    route:
    - destination:
        host: {{ .ServiceName }}.{{ .Namespace }}.svc.cluster.local
        port:
          number: 7681
`

	KubeconfigTmplV1 = `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: {{ .CAData }}
    server: {{ .Server }}
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: cloudtty-controller-manager
  name: cloudtty-controller-manager@kubernetes
current-context: cloudtty-controller-manager@kubernetes
kind: Config
users:
- name: cloudtty-controller-manager
  user: 
    token: {{ .Token }}
`
)
