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
	PodTmplV1 = `
apiVersion: v1
kind: Pod
metadata:
  namespace: "{{ .Namespace }}"
  name: "{{ .Name }}"
spec:
  containers:
  - name: web-tty
    image: "{{ .Image }}"
    imagePullPolicy: IfNotPresent
    command:
    - /bin/bash
    - -ec
    - |
      while :; do sleep 2073600; done
    ports:
    - containerPort: 7681
      name: tty-ui
      protocol: TCP
    {{- if .Resources }}
    resources:
      {{- if .Resources.Requests }}
      requests:
        cpu: {{ .Resources.Requests.Cpu}}
        memory: {{ .Resources.Requests.Memory}}
      {{- end }}
      {{- if .Resources.Limits }}
      limits:
        cpu: {{ .Resources.Limits.Cpu}}
        memory: {{ .Resources.Limits.Memory}}
      {{- end }}
    {{- end }}
  nodeSelector:
  {{range $key, $value := .NodeSelector}}
    {{ $key }}: "{{ $value }}"
  {{end}}
  restartPolicy: Never
`

	ServiceTmplV1 = `
apiVersion: v1
kind: Service
metadata:
  name: "{{ .Name }}"
  namespace: "{{ .Namespace }}"
spec:
  ports:
  - name: ttyd
    port: 7681
    protocol: TCP
    targetPort: 7681
  selector:
    worker.cloudtty.io/name: "{{ .Worker }}"
  type: {{ .Type }}
`

	IngressTmplV1 = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: "{{ .Name }}"
  namespace: "{{ .Namespace }}"
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    {{- range $key, $value := .Annotations }}
    {{ $key }}: "{{ $value }}"
    {{- end }}
spec:
  ingressClassName: "{{ .IngressClassName }}"
  rules:
  - http:
      paths:
      - path: "{{ .Path }}"
        pathType: Prefix
        backend:
          service:
            name: "{{ .ServiceName }}"
            port:
              number: 7681
`

	VirtualServiceV1Beta1 = `
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: "{{ .Name }}"
  namespace: "{{ .Namespace }}"
spec:
  exportTo:
  - "{{ .ExportTo }}"
  gateways:
  - "{{ .Gateway }}"
  hosts:
  - '*'
  http:
  - match:
    - uri:
        prefix: "{{ .Path }}"
    rewrite:
      uri: /
    route:
    - destination:
        host: "{{ .ServiceName }}.{{ .Namespace }}.svc.cluster.local"
        port:
          number: 7681
`

	KubeconfigTmplV1 = `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: "{{ .CAData }}"
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
    token: "{{ .Token }}"
`
)
