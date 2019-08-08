package metallb

const namespace = `
apiVersion: v1
kind: Namespace
metadata:
  name: metallb-system
  labels:
    app: metallb
`

const serviceAccountController = `
apiVersion: v1
kind: ServiceAccount
metadata:
  namespace: metallb-system
  name: controller
  labels:
    app: metallb
`

const serviceAccountSpeaker = `
apiVersion: v1
kind: ServiceAccount
metadata:
  namespace: metallb-system
  name: speaker
  labels:
    app: metallb
`
const clusterRoleMetallbSystemController = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: metallb-system:controller
  labels:
    app: metallb
rules:
- apiGroups: [""]
  resources: ["services"]
  verbs: ["get", "list", "watch", "update"]
- apiGroups: [""]
  resources: ["services/status"]
  verbs: ["update"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]
`

// Note: Diversion from upstream.
// This ClusterRole has added rule to use the Pod Security Policy.
const clusterRoleMetallbSystemSpeaker = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: metallb-system:speaker
  labels:
    app: metallb
rules:
- apiGroups: [""]
  resources: ["services", "endpoints", "nodes"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["extensions"]
  resources: ["podsecuritypolicies"]
  resourceNames: ["metallb-speaker"]
  verbs: ["use"]
`

const roleConfigWatcher = `
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: metallb-system
  name: config-watcher
  labels:
    app: metallb
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create"]
`

const clusterRoleBindingMetallbSystemController = `
## Role bindings
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: metallb-system:controller
  labels:
    app: metallb
subjects:
- kind: ServiceAccount
  name: controller
  namespace: metallb-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: metallb-system:controller
`

const clusterRoleBindingMetallbSystemSpeaker = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: metallb-system:speaker
  labels:
    app: metallb
subjects:
- kind: ServiceAccount
  name: speaker
  namespace: metallb-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: metallb-system:speaker
`

const roleBindingConfigWatcher = `
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  namespace: metallb-system
  name: config-watcher
  labels:
    app: metallb
subjects:
- kind: ServiceAccount
  name: controller
- kind: ServiceAccount
  name: speaker
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: config-watcher
`

const deploymentController = `
apiVersion: apps/v1beta2
kind: Deployment
metadata:
  namespace: metallb-system
  name: controller
  labels:
    app: metallb
    component: controller
spec:
  revisionHistoryLimit: 3
  selector:
    matchLabels:
      app: metallb
      component: controller
  template:
    metadata:
      labels:
        app: metallb
        component: controller
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "7472"
    spec:
      {{- if .ControllerNodeSelectors }}
      nodeSelector:
        {{- range $key, $value := .ControllerNodeSelectors }}
        {{ $key }}: "{{ $value }}"
        {{- end }}
      {{- end }}
      serviceAccountName: controller
      terminationGracePeriodSeconds: 0
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534 # nobody
      containers:
      - name: controller
        image: metallb/controller:v0.7.3
        imagePullPolicy: IfNotPresent
        args:
        - --port=7472
        - --config=config
        ports:
        - name: monitoring
          containerPort: 7472
        resources:
          limits:
            cpu: 100m
            memory: 100Mi

        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - all
          readOnlyRootFilesystem: true
      {{- if .ControllerTolerations }}
      tolerations: {{ .ControllerTolerations }}
      {{- end }}
`

const daemonsetSpeaker = `
apiVersion: apps/v1beta2
kind: DaemonSet
metadata:
  namespace: metallb-system
  name: speaker
  labels:
    app: metallb
    component: speaker
spec:
  selector:
    matchLabels:
      app: metallb
      component: speaker
  template:
    metadata:
      labels:
        app: metallb
        component: speaker
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "7472"
    spec:
      {{- if .SpeakerNodeSelectors }}
      nodeSelector:
        {{- range $key, $value := .SpeakerNodeSelectors }}
        {{ $key }}: "{{ $value }}"
        {{- end }}
      {{- end }}
      serviceAccountName: speaker
      terminationGracePeriodSeconds: 0
      hostNetwork: true
      containers:
      - name: speaker
        image: metallb/speaker:v0.7.3
        imagePullPolicy: IfNotPresent
        args:
        - --port=7472
        - --config=config
        env:
        - name: METALLB_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        ports:
        - name: monitoring
          containerPort: 7472
        resources:
          limits:
            cpu: 100m
            memory: 100Mi

        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop:
            - all
            add:
            - net_raw
      {{- if .SpeakerTolerations }}
      tolerations: {{ .SpeakerTolerations }}
      {{- end }}
`

// Note: Diversion from upstream.
// This config was created specifically for clusters that has Pod Security Policy enabled on them.
const pspMetallbSpeaker = `
apiVersion: extensions/v1beta1
kind: PodSecurityPolicy
metadata:
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: docker/default
    seccomp.security.alpha.kubernetes.io/defaultProfileName: docker/default
  name: metallb-speaker
spec:
  hostNetwork: true
  hostPorts:
  - min: 7472
    max: 7472
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  allowedCapabilities:
  - net_raw
  requiredDropCapabilities:
  - all
  seLinux:
    rule: RunAsAny
  fsGroup:
    ranges:
    - max: 65535
      min: 1
    rule: MustRunAs
  runAsUser:
    # Require the container to run without root privileges.
    rule: RunAsAny
  supplementalGroups:
    ranges:
    - max: 65535
      min: 1
    rule: MustRunAs
  volumes:
  - secret
`
