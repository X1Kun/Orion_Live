# --- Build Stage (构建阶段) ---
# 1. 选择基础镜像：使用官方的 golang:1.24-alpine 作为构建环境。
#    - `golang:1.24`: 提供了完整的 Go 1.24 SDK，用于编译我们的代码。
#    - `alpine`: 这是一个极小化的 Linux 发行版，使得构建环境本身就很小，下载更快。
#    - `AS builder`: 给这个构建阶段起一个别名，叫 "builder"。这在后续的多阶段构建中至关重要，相当于一个“临时工作台”的标签。
FROM golang:1.24-alpine AS builder

# 2. 设置工作目录：在构建镜像内部创建一个名为 /app 的工作目录。
#    后续的所有命令（如 COPY, RUN）都将默认在这个目录下执行。
WORKDIR /app

# 3. 复制依赖描述文件：将 go.mod 和 go.sum 文件复制到工作目录中。
#    go.mod 定义了项目依赖，go.sum 锁定了依赖版本。
COPY go.mod go.sum ./

# 4. 设置Go环境变量：
#    - `ENV GO111MODULE=on`: 明确开启 Go Modules 模式。
#    - `ENV GOPROXY=...`: 设置 Go 模块代理，使用国内镜像加速依赖包的下载。
ENV GO111MODULE=on
ENV GOPROXY=https://goproxy.cn,direct

# 5. 下载依赖（缓存优化关键步骤）：
#    这一步非常巧妙！因为它只依赖于 go.mod 和 go.sum 文件。根据 Docker 的分层缓存机制，
#    只要这两个文件不发生变化，这一层就会被缓存。下次构建时，如果源代码变了但依赖没变，
#    Docker 会直接使用缓存，跳过耗时的下载过程，极大地提升了构建速度。
RUN go mod download

# 6. 复制所有源代码：
#    将项目的所有文件（源代码、配置文件模板等）都复制到工作目录中。
#    这一步放在 `go mod download` 之后，也是为了优化缓存。只有当我们修改了源代码时，
#    才会让这一层及之后的缓存失效，而不会影响前面的依赖下载层。
COPY . .

# 7. 编译应用程序：
#    - `CGO_ENABLED=0`: 禁用CGO。这会构建一个纯静态链接的、不依赖任何系统C库的二进制文件。
#      这对于在极简的`alpine`最终镜像中运行至关重要，因为`alpine`缺少很多GNU/Linux发行版中常见的`glibc`库。
#    - `GOOS=linux GOARCH=amd64`: 交叉编译指令，告诉Go编译器，即使当前是在macOS或Windows上构建，
#      目标产物也必须是能在 Linux amd64 架构上运行的。
#    - `go build -o /app/main ./cmd/server`: 编译程序，并将输出的可执行文件命名为 `main`，放在 `/app` 目录下。
#      `./cmd/server` 是你的主程序入口包。
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/main ./cmd/server

# --- Final Stage (最终阶段) ---
# 8. 开启一个全新的、干净的阶段：
#    使用极小化的 `alpine:latest` 镜像作为最终的运行环境。这个镜像只有几MB大小，
#    不包含任何Go编译器或源代码，非常安全和轻量。我们在这里“扔掉了”整个 `builder` 工作台。
FROM alpine:latest

# 9. 从构建阶段拷贝产物：
#    - `--from=builder`: 指定从名为 "builder" 的阶段进行拷贝。
#    - `/app/main`: 源文件路径（在 builder 阶段）。
#    - `/main`: 目标文件路径（在当前阶段）。
#    这是多阶段构建的“魔法”所在！我们只“捡走”了最终需要运行的程序，而抛弃了所有中间的编译工具、依赖包和源代码。
COPY --from=builder /app/main /main

# 10. 安装运行时依赖：
#     - `ca-certificates`: 提供了根证书，如果你的程序需要发起HTTPS请求或与其他需要TLS的服务通信，这是必需的。
#     - `tzdata`: 提供了时区数据库，让下面的 `ENV TZ` 设置能够生效。
RUN apk --no-cache add ca-certificates tzdata

# 11. 设置时区：
#     将容器的环境变量 `TZ` 设置为上海时区。
ENV TZ=Asia/Shanghai

# 12. 声明端口：
#     这只是一个元数据声明，告诉用户（和某些自动化工具）这个容器内的服务打算在8080端口上监听。
#     它本身不会自动发布（映射）端口到宿主机。真正的端口发布是在 `docker-compose.yml` 的 `ports` 指令中完成的。
EXPOSE 8080

# 13. 定义启动命令：
#     - `ENTRYPOINT`: 定义容器启动时要执行的默认命令。
#     - `["/main"]`: 当容器启动时，它会执行 `/main` 这个程序。使用JSON数组格式是推荐的做法，可以避免shell解析带来的问题。
ENTRYPOINT ["/main"]