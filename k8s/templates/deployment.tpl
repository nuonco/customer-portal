---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "common.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "common.labels" . | nindent 4 }}
spec:
  {{- if ne .Values.environment "prod" }}
  strategy:
    rollingUpdate:
      maxSurge: 0
      maxUnavailable: 1
  {{- end }}
  selector:
    matchLabels:
      {{- include "common.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "common.selectorLabels" . | nindent 8 }}
        tags.datadoghq.com/service: customer-dashboard
      annotations:
        rollme: {{ randAlphaNum 5 | quote }}
        ad.datadoghq.com/tags: '{"service_type":"app", "service_deployment":"customer-dashboard"}'

    spec:
      nodeSelector:
        pool.nuon.co: "public"
      tolerations:
        - key: "pool.nuon.co/public"
          operator: "Exists"
          effect: "NoSchedule"
        - key: "pool.nuon.co"
          operator: "Equal"
          value: "public"
          effect: "NoSchedule"
      topologySpreadConstraints:
        {{- if ne .Values.environment "prod" }}
        - maxSkew: 1
          topologyKey: "kubernetes.io/hostname"
          whenUnsatisfiable: DoNotSchedule
          minDomains: 2
          labelSelector:
            matchLabels:
              {{- include "common.selectorLabels" . | nindent 14 }}
        {{- end }}
        - maxSkew: 1
          topologyKey: "topology.kubernetes.io/zone"
          whenUnsatisfiable: ScheduleAnyway
          labelSelector:
            matchLabels:
              {{- include "common.selectorLabels" . | nindent 14 }}
      serviceAccountName: {{ .Values.serviceAccount.name }}
      automountServiceAccountToken: true
      containers:
        - name: {{ include "common.fullname" . }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          command:
            - /bin/service
          ports:
            - name: http
              containerPort: {{ .Values.app.port }}
              protocol: TCP
          readinessProbe:
            failureThreshold: 10
            httpGet:
              path: {{ .Values.app.readiness_probe}}
              port: http
          livenessProbe:
            failureThreshold: 10
            httpGet:
              path: {{ .Values.app.liveness_probe}}
              port: http
          resources:
            limits:
              cpu: {{ .Values.app.resources.limits.cpu }}
              memory: {{ .Values.app.resources.limits.memory }}
            requests:
              cpu: {{ .Values.app.resources.requests.cpu }}
              memory: {{ .Values.app.resources.requests.memory }}
          envFrom:
            - configMapRef:
                name: {{ include "common.fullname" . }}
          env:
          {{- range $envSecret := .Values.envSecrets }}
            - name: {{ $envSecret.name }}
              valueFrom:
                secretKeyRef:
                  name: {{ $envSecret.valueFrom.name }}
                  key: {{ $envSecret.valueFrom.key }}
          {{- end}}
            - name: HOST_IP
              valueFrom:
                  fieldRef:
                      fieldPath: status.hostIP
            - name: HOST_NAME
              valueFrom:
                  fieldRef:
                      fieldPath: spec.nodeName
            - name: DD_SERVICE
              value: customer-dashboard
            - name: SERVICE_TYPE
              value: app
            - name: SERVICE_DEPLOYMENT
              value: customer-dashboard
{{- if eq .Values.environment "prod" }}
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: {{ include "common.fullname" . }}
  namespace: {{ .Release.Namespace }}
spec:
  minAvailable: 1
  selector:
    matchLabels:
      {{- include "common.selectorLabels" . | nindent 6 }}
{{- end }}
