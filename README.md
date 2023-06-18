# sidecar-proxy
A simple HTTP sidecar-proxy implementing basic authentication.

*privides a basic auth access for services like stable-diffusion-webui and jaeger, supports tls,*
*password hashing(md5 and bcrypt).*


#### 1. docker images
- registry.cn-shanghai.aliyuncs.com/d2jvkpn/sidecar-proxy:dev


#### 2. serve
- configuration(configs/sidecar_proxy.yaml):
```yaml
sidecar_proxy:
  service: http://127.0.0.1:8000
  cors: "*"
  # pass_with_prefix: ["GET@/assets/"]
  pass_with_prefix: []
  tls: false
  cert: "configs/server.cert"
  key: "configs/server.key"
  limit_ips: []
  insert_headers:
  - { key: "x-1", value: "y-1" }
  - { key: "x-2", value: "y-2" }
  basic_auth:
    method: md5
    users:
    - { username: hello, password: 6de41d334b7ce946682da48776a10bb9 }
    # method: bcrypt
    # users:
    # - {username: hello, password: "$2a$10$scqefoWP3SwzgB.bLkbQ0e0Cre45AA16ibI3lxichOp3FohzQm9BK" }
```

- commandline:
```bash
go run main.go serve --config=configs/sidecar-proxy.yaml --addr=:9000
```
*implements a basic auth througth 127.0.0.0:9000 for the local service 127.0.0.1:8000*


##### 3. create-user
```bash
go run main.go create-user --method md5

go run main.go create-user --method=bcrypt --cost=10
```
