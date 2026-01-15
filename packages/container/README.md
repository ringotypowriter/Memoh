# @memoh/container

基于 nerdctl (containerd) 的容器化工具包，提供简单易用的容器管理 API。

## 特性

- 🚀 基于 nerdctl 的现代容器管理（Docker 兼容）
- 📦 简洁的 API 设计
- 🔧 完整的容器生命周期管理
- 📝 TypeScript 支持
- 🎯 命名空间隔离

## 安装

```bash
pnpm install @memoh/container
```

## 前置要求

### macOS (推荐使用 Lima)

```bash
# 安装 Lima
brew install lima

# 启动 Lima（已包含 nerdctl）
limactl start

# 验证
lima nerdctl version
```

### Linux

```bash
# 安装 nerdctl
# 参考: https://github.com/containerd/nerdctl/releases

# 或使用包管理器
brew install nerdctl  # Homebrew on Linux
```

**详细 macOS 配置请参考 [NERDCTL_SETUP.md](./NERDCTL_SETUP.md)**

## 快速开始

### 创建容器

使用 `createContainer` 创建一个新容器：

```typescript
import { createContainer } from '@memoh/container';

const container = await createContainer({
  name: 'my-nginx',
  image: 'docker.io/library/nginx:latest',
  env: {
    PORT: '8080',
    NODE_ENV: 'production',
  },
});

console.log('Container created:', container.id);
console.log('Status:', container.status);
```

### 操作容器

使用 `useContainer` 获取容器操作方法：

```typescript
import { useContainer } from '@memoh/container';

const container = useContainer('my-nginx');

// 启动容器
await container.start();

// 获取容器信息
const info = await container.info();
console.log('Container status:', info.status);

// 执行命令
const result = await container.exec(['nginx', '-v']);
console.log('Output:', result.stdout);

// 查看日志
const logs = await container.logs();
console.log(logs);

// 暂停容器
await container.pause();

// 恢复容器
await container.resume();

// 停止容器
await container.stop(10); // 10秒超时

// 删除容器
await container.remove();
```

## API 文档

### createContainer

创建并返回容器信息。

```typescript
function createContainer(
  config: ContainerConfig,
  options?: ContainerdOptions
): Promise<ContainerInfo>
```

**参数：**

- `config.name` - 容器名称（必需）
- `config.image` - 镜像引用（必需）
- `config.command` - 容器启动命令
- `config.env` - 环境变量
- `config.workingDir` - 工作目录
- `config.namespace` - 命名空间（默认：default）
- `config.labels` - 容器标签

**返回：** `ContainerInfo` 对象

### useContainer

获取容器操作方法。

```typescript
function useContainer(
  containerIdOrName: string,
  options?: ContainerdOptions
): ContainerOperations
```

**返回的操作方法：**

- `start()` - 启动容器
- `stop(timeout?)` - 停止容器
- `restart(timeout?)` - 重启容器
- `pause()` - 暂停容器
- `resume()` - 恢复容器
- `remove(force?)` - 删除容器
- `exec(command)` - 执行命令
- `info()` - 获取容器信息
- `logs(follow?)` - 获取日志
- `stats()` - 获取统计信息

### listContainers

列出所有容器。

```typescript
function listContainers(options?: ContainerdOptions): Promise<ContainerInfo[]>
```

**示例：**

```typescript
import { listContainers } from '@memoh/container';

const containers = await listContainers();
for (const container of containers) {
  console.log(`${container.name}: ${container.status}`);
}
```

### containerExists

检查容器是否存在。

```typescript
function containerExists(
  containerIdOrName: string,
  options?: ContainerdOptions
): Promise<boolean>
```

### removeAllContainers

删除所有容器。

```typescript
function removeAllContainers(
  force?: boolean,
  options?: ContainerdOptions
): Promise<void>
```

## 高级用法

### 自定义命名空间

```typescript
import { createContainer, useContainer } from '@memoh/container';

// 在自定义命名空间中创建容器
const container = await createContainer(
  {
    name: 'my-app',
    image: 'docker.io/library/node:18',
  },
  {
    namespace: 'production',
  }
);

// 操作同一命名空间的容器
const ops = useContainer('my-app', { namespace: 'production' });
await ops.start();
```

### 自定义 Socket 路径

```typescript
const container = useContainer('my-app', {
  socket: '/custom/path/to/containerd.sock',
});
```

### 完整示例：Web 服务部署

```typescript
import { createContainer, useContainer } from '@memoh/container';

async function deployWebService() {
  // 创建容器
  const container = await createContainer({
    name: 'web-service',
    image: 'docker.io/library/nginx:alpine',
    env: {
      NGINX_PORT: '8080',
    },
    labels: {
      app: 'web-service',
      version: '1.0.0',
    },
  });

  console.log('Container created:', container.id);

  // 启动容器
  const ops = useContainer(container.name);
  await ops.start();
  console.log('Container started');

  // 等待服务就绪
  await new Promise(resolve => setTimeout(resolve, 2000));

  // 检查状态
  const info = await ops.info();
  console.log('Status:', info.status);

  // 执行健康检查
  const health = await ops.exec(['curl', '-f', 'http://localhost:8080']);
  if (health.exitCode === 0) {
    console.log('Service is healthy');
  }

  return ops;
}

// 使用
const service = await deployWebService();

// 稍后停止服务
await service.stop();
await service.remove();
```

## 类型定义

```typescript
interface ContainerConfig {
  name: string;
  image: string;
  command?: string[];
  env?: Record<string, string>;
  workingDir?: string;
  network?: string;
  mounts?: Mount[];
  labels?: Record<string, string>;
  namespace?: string;
}

interface ContainerInfo {
  id: string;
  name: string;
  image: string;
  status: ContainerStatus;
  namespace: string;
  createdAt: Date;
  labels?: Record<string, string>;
}

type ContainerStatus = 'created' | 'running' | 'paused' | 'stopped' | 'unknown';

interface ExecResult {
  exitCode: number;
  stdout: string;
  stderr: string;
}
```

## 注意事项

1. **权限要求**：操作容器通常需要 root 权限或将用户添加到适当的组
2. **containerd 服务**：确保 containerd 服务正在运行
3. **镜像拉取**：首次使用镜像时会自动拉取，可能需要一些时间
4. **命名空间**：不同命名空间的容器相互隔离
5. **清理资源**：使用完容器后记得清理（stop + remove）

## 故障排查

### 命令未找到

如果遇到 `ctr command not found` 错误：

```bash
# 检查 containerd 是否安装
which ctr

# 安装 containerd
brew install containerd  # macOS
apt-get install containerd  # Linux
```

### 权限被拒绝

如果遇到权限错误：

```bash
# 将用户添加到 docker 组（如果存在）
sudo usermod -aG docker $USER

# 或者使用 sudo 运行你的程序
sudo node your-script.js
```

### 容器无法启动

检查容器日志：

```typescript
const container = useContainer('my-container');
const logs = await container.logs();
console.log(logs);
```

## 开发

```bash
# 安装依赖
pnpm install

# 运行测试
pnpm test

# 构建
pnpm build
```

## 许可证

MIT
