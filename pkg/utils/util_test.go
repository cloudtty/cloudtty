package utils

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cloudtty/cloudtty/pkg/manifests"
	"github.com/google/go-cmp/cmp"
)

func TestParseTemplate(t *testing.T) {
	type args struct {
		strtmpl string
		obj     interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "test miss required field",
			args: args{
				strtmpl: manifests.JobTmplV1,
				obj: struct {
					Name    string
					Command string
					Secret  string
					Ttl     int32
				}{
					Name:    "test-tty",
					Command: "sleep 100",
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "test parse job template",
			args: args{
				strtmpl: manifests.JobTmplV1,
				obj: struct {
					Namespace string
					Name      string
					Command   string
					Secret    string
					Once      bool
					UrlArg    bool
					Ttl       int32
				}{
					Namespace: "test-ns",
					Name:      "test-tty",
					Command:   "sleep 100",
					Secret:    ``,
					Once:      false,
					Ttl:       10,
				},
			},
			want: `
  apiVersion: batch/v1
  kind: Job
  metadata:
    namespace: test-ns
    name: test-tty
  spec:
    ttlSecondsAfterFinished: 30
    template:
      spec:
        automountServiceAccountToken: false
        containers:
        - name: web-tty
          image: "ghcr.io/cloudtty/cloudshell:v0.5.2"
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
              if [ -f /usr/lib/ttyd/index.html ]; then index=" --index /usr/lib/ttyd/index.html ";fi
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
            value: "false"
          - name: TTL
            value: "10"
          - name: COMMAND
            value: sleep 100
          - name: URLARG
            value: "false"
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
        - secret:
            defaultMode: 420
            secretName: 
          name: kubeconfig
			`,
			wantErr: false,
		},
		{
			name: "test parse service template",
			args: args{
				strtmpl: manifests.ServiceTmplV1,
				obj: struct {
					Name      string
					Namespace string
					JobName   string
					Type      string
				}{
					Name:      "cloudshell-test",
					Namespace: "test-ns",
					JobName:   "cloudshell-test",
					Type:      "ClusterIP",
				},
			},
			wantErr: false,
			want: `
	apiVersion: v1
	kind: Service
	metadata:
	  name: cloudshell-test
	  namespace: test-ns
	spec:
	  ports:
	  - name: ttyd
	    port: 7681
	    protocol: TCP
	    targetPort: 7681
	  selector:
	    job-name: cloudshell-test
	  type: ClusterIP
			`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTemplate(tt.args.strtmpl, tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			get := strings.Replace(string(got), "\t", "", -1)
			want := strings.Replace(tt.want, "\t", "", -1)
			if !reflect.DeepEqual(get, want) {
				t.Errorf("ParseTemplate() diff:%v", cmp.Diff(string(got), tt.want))
			}
		})
	}
}
