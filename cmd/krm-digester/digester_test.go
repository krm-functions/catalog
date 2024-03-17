package main

import (
	"testing"

	"github.com/michaelvl/krm-functions/pkg/helm"
	"github.com/stretchr/testify/assert"
)

func TestDigester(t *testing.T) {
	testCases := []struct {
		TestName       string
		FunctionConfig string
		Input          string
		ExpectedOutput string
	}{
		{
			TestName: "locate images",
			FunctionConfig: `
`,
			Input: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: fluentd-elasticsearch
  namespace: kube-system
  labels:
    k8s-app: fluentd-logging
spec:
  selector:
    matchLabels:
      name: fluentd-elasticsearch
  template:
    metadata:
      labels:
        name: fluentd-elasticsearch
    spec:
      tolerations:
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      containers:
      - name: fluentd-elasticsearch
        image: quay.io/fluentd_elasticsearch/fluentd:v2.5.2
        resources:
          limits:
            memory: 200Mi
          requests:
            cpu: 100m
            memory: 200Mi
---
apiVersion: batch/v1
kind: Job
metadata:
  name: pi
spec:
  template:
    spec:
      containers:
      - name: pi
        image: perl:5.34.0
        command: ["perl",  "-Mbignum=bpi", "-wle", "print bpi(2000)"]
      restartPolicy: Never
  backoffLimit: 4
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: hello
spec:
  schedule: "* * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: hello
            image: busybox:1.28
            imagePullPolicy: IfNotPresent
            command:
            - /bin/sh
            - -c
            - date; echo Hello from the Kubernetes cluster
          restartPolicy: OnFailure
---
apiVersion: apps/v1
kind: ReplicaSet
metadata:
  name: frontend
  labels:
    app: guestbook
    tier: frontend
spec:
  # modify replicas according to your case
  replicas: 3
  selector:
    matchLabels:
      tier: frontend
  template:
    metadata:
      labels:
        tier: frontend
    spec:
      containers:
      - name: php-redis
        image: us-docker.pkg.dev/google-samples/containers/gke/gb-frontend:v5
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: web
spec:
  selector:
    matchLabels:
      app: nginx # has to match .spec.template.metadata.labels
  serviceName: "nginx"
  replicas: 3 # by default is 1
  minReadySeconds: 10 # by default is 0
  template:
    metadata:
      labels:
        app: nginx # has to match .spec.selector.matchLabels
    spec:
      terminationGracePeriodSeconds: 10
      containers:
      - name: nginx
        image: registry.k8s.io/nginx-slim:0.8
        ports:
        - containerPort: 80
          name: web
---
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: helloworld-go
  namespace: default
spec:
  template:
    spec:
      containers:
        - image: ghcr.io/knative/helloworld-go:latest
          env:
            - name: TARGET
              value: "Go Sample v1"
---
apiVersion: v1
kind: Pod
metadata:
  name: myapp-pod
  labels:
    app.kubernetes.io/name: MyApp
spec:
  containers:
  - name: myapp-container
    image: busybox:1.29
    command: ['sh']
  initContainers:
  - name: init-myservice
    image: busybox:1.30
    command: ['sh']
  - name: init-mydb
    image: busybox:1.31
    command: ['sh']
`,
			ExpectedOutput: ``,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.TestName, func(t *testing.T) {
			objs, err := helm.ParseAsRNodes([]byte(tc.Input))
			if err != nil {
				t.Fatal(err)
			}
			imageFilter := NewImageFilter()
			_, err = imageFilter.Filter(objs)
			if err != nil {
				t.Fatal()
			}
			assert.Equal(t, 10, len(imageFilter.Images))
			assert.Equal(t, "nginx:1.14.2", imageFilter.Images[0])
			assert.Equal(t, "quay.io/fluentd_elasticsearch/fluentd:v2.5.2", imageFilter.Images[1])
			assert.Equal(t, "perl:5.34.0", imageFilter.Images[2])
			assert.Equal(t, "busybox:1.28", imageFilter.Images[3])
			assert.Equal(t, "us-docker.pkg.dev/google-samples/containers/gke/gb-frontend:v5", imageFilter.Images[4])
			assert.Equal(t, "registry.k8s.io/nginx-slim:0.8", imageFilter.Images[5])
			assert.Equal(t, "ghcr.io/knative/helloworld-go:latest", imageFilter.Images[6])
			assert.Equal(t, "busybox:1.29", imageFilter.Images[7])
			assert.Equal(t, "busybox:1.30", imageFilter.Images[8])
			assert.Equal(t, "busybox:1.31", imageFilter.Images[9])
		})
	}
}
