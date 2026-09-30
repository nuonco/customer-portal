---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "common.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "common.labels" . | nindent 4 }}
  annotations:
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTP":80},{"HTTPS":443}]'
    alb.ingress.kubernetes.io/ssl-redirect: '443'
    alb.ingress.kubernetes.io/certificate-arn: {{ .Values.app.alb.public_domain_certificate }}
    alb.ingress.kubernetes.io/aws-load-balancer-ssl-ports: https
    alb.ingress.kubernetes.io/healthcheck-path: /livez
    alb.ingress.kubernetes.io/healthcheck-interval-seconds: '5'
    alb.ingress.kubernetes.io/healthcheck-timeout-seconds: '2'
    alb.ingress.kubernetes.io/unhealthy-threshold-count: '2'
    alb.ingress.kubernetes.io/healthy-threshold-count: '2'
    alb.ingress.kubernetes.io/load-balancer-attributes: idle_timeout.timeout_seconds=300
    {{ if .Values.app.alb.group_name }}
    alb.ingress.kubernetes.io/group.name: {{ .Values.app.alb.group_name }}
    alb.ingress.kubernetes.io/ssl-policy: ELBSecurityPolicy-TLS13-1-0-2021-06
    alb.ingress.kubernetes.io/tags: {{ .Values.app.alb.group_tags | quote }}
    {{ else }}
    alb.ingress.kubernetes.io/tags: 'service=customer-dashboard,service_type=app,env={{ .Values.env.ENV }}'
    {{ end }}
    external-dns.alpha.kubernetes.io/hostname: {{ .Values.app.alb.public_domain }},*.{{ .Values.app.alb.public_domain }}
spec:
  ingressClassName: alb
  rules:
    - host: "{{ .Values.app.alb.public_domain }}"
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ include "common.fullname" . }}
                port:
                  name: http
    - host: "*.{{ .Values.app.alb.public_domain }}"
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ include "common.fullname" . }}
                port:
                  name: http
---
apiVersion: v1
kind: Service
metadata:
  name: {{ include "common.fullname" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "common.labels" . | nindent 4 }}
spec:
  selector:
    {{- include "common.selectorLabels" . | nindent 4 }}
  type: ClusterIP
  ports:
    - name: http
      port: 8080
      targetPort: http
