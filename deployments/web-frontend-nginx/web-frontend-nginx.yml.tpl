apiVersion: v1
kind: Service
metadata:
  name: web-frontend-nginx
  namespace: kube-system
spec:
  ports:
  - port: 8995
    targetPort: 8995
  type: ClusterIP
  selector:
    app: web-frontend-nginx
---

apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: web-frontend-nginx
  namespace: kube-system
  labels:
    app: web-frontend-nginx
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web-frontend-nginx
  template:
    metadata:
      labels:
        app: web-frontend-nginx
    spec:
      terminationGracePeriodSeconds: 0
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      imagePullSecrets:
      - name: _HARBOR_SECRET_NAME_
      nodeSelector:
        monitor: deploy
      containers:
      - image: _HARBOR_IMAGE_ADDR_/ote-unified-fe:v0.1.14
        imagePullPolicy: IfNotPresent
        name: web-frontend-nginx
        ports:
        - containerPort: 8995
          protocol: TCP
        volumeMounts:
        - mountPath: /etc/nginx/conf.d
          name: web-frontend-nginx-conf
      volumes:
      - name: web-frontend-nginx-conf
        configMap:
          defaultMode: 420
          name: web-frontend-nginx-conf
---

apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app: web-frontend-nginx
  name: web-frontend-nginx-conf
  namespace: kube-system
data:
  nginx.conf: |
    server {
        listen   8995;
        server_name  web_frontend_nginx;
        # disable any limits to avoid HTTP 413 for large image uploads
        client_max_body_size 0;
        # required to avoid HTTP 411: see Issue #1486 (https://github.com/docker/docker/issues/1486)
        chunked_transfer_encoding on;
        location / {
            root   /usr/share/nginx/html;
            index  index.html index.htm;
            try_files $uri $uri/ /index.html?$args;
        }
        error_page   500 502 503 504  /50x.html;
        location = /50x.html {
            root   /usr/share/nginx/html;
        }
        location /api {
            proxy_pass          http://open-api:8012;
            proxy_buffering     off;
            #proxy_buffer_size   128k;
            #proxy_buffers       100 128k;
            proxy_next_upstream off;
            proxy_read_timeout  120s;
            proxy_ssl_verify  off;
            proxy_pass_header   Server;
            proxy_set_header    Host                $http_host;
            proxy_set_header    X-Real-IP           $remote_addr;
            proxy_set_header    X-Forwarded-For     $proxy_add_x_forwarded_for;
            proxy_set_header    X-Forwarded-Proto   https;
            proxy_redirect off;
        }
        location /v1 {
            proxy_pass          http://open-api:8012;
            proxy_buffering     off;
            #proxy_buffer_size   128k;
            #proxy_buffers       100 128k;
            proxy_next_upstream off;
            proxy_read_timeout  120s;
            proxy_ssl_verify  off;
            proxy_pass_header   Server;
            proxy_set_header    Host                $http_host;
            proxy_set_header    X-Real-IP           $remote_addr;
            proxy_set_header    X-Forwarded-For     $proxy_add_x_forwarded_for;
            proxy_set_header    X-Forwarded-Proto   https;
            proxy_redirect off;
        }
    }
