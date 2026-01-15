/**
 * Container runtime types and interfaces
 */

/**
 * Container configuration options
 */
export interface ContainerConfig {
  /** Container name/ID */
  name: string;
  /** Container image reference */
  image: string;
  /** Command to run in the container */
  command?: string[];
  /** Environment variables */
  env?: Record<string, string>;
  /** Working directory */
  workingDir?: string;
  /** Network namespace */
  network?: string;
  /** Mount points */
  mounts?: Mount[];
  /** Labels for the container */
  labels?: Record<string, string>;
  /** Container namespace (default: "default") */
  namespace?: string;
}

/**
 * Mount configuration
 */
export interface Mount {
  /** Mount type: bind, volume, tmpfs */
  type: 'bind' | 'volume' | 'tmpfs';
  /** Source path (host) */
  source: string;
  /** Target path (container) */
  target: string;
  /** Read-only mount */
  readonly?: boolean;
}

/**
 * Container information
 */
export interface ContainerInfo {
  /** Container ID */
  id: string;
  /** Container name */
  name: string;
  /** Container image */
  image: string;
  /** Container status */
  status: ContainerStatus;
  /** Container namespace */
  namespace: string;
  /** Creation timestamp */
  createdAt: Date;
  /** Labels */
  labels?: Record<string, string>;
}

/**
 * Container status
 */
export type ContainerStatus = 'created' | 'running' | 'paused' | 'stopped' | 'unknown';

/**
 * Container execution result
 */
export interface ExecResult {
  /** Exit code */
  exitCode: number;
  /** Standard output */
  stdout: string;
  /** Standard error */
  stderr: string;
}

/**
 * Container stats
 */
export interface ContainerStats {
  /** CPU usage percentage */
  cpuUsage: number;
  /** Memory usage in bytes */
  memoryUsage: number;
  /** Memory limit in bytes */
  memoryLimit: number;
  /** Network I/O */
  networkIO?: {
    rxBytes: number;
    txBytes: number;
  };
}

/**
 * Container operations interface
 */
export interface ContainerOperations {
  /** Build exec command */
  buildExecCommand(command: string[]): string[];
  /** Start the container */
  start(): Promise<void>;
  /** Stop the container */
  stop(timeout?: number): Promise<void>;
  /** Restart the container */
  restart(timeout?: number): Promise<void>;
  /** Pause the container */
  pause(): Promise<void>;
  /** Resume the container */
  resume(): Promise<void>;
  /** Remove the container */
  remove(force?: boolean): Promise<void>;
  /** Execute a command in the container */
  exec(command: string[]): Promise<ExecResult>;
  /** Get container info */
  info(): Promise<ContainerInfo>;
  /** Get container logs */
  logs(follow?: boolean): Promise<string>;
  /** Get container stats */
  stats(): Promise<ContainerStats>;
}

/**
 * Containerd client options
 */
export interface ContainerdOptions {
  /** Containerd socket path */
  socket?: string;
  /** Containerd namespace */
  namespace?: string;
  /** Timeout for operations (ms) */
  timeout?: number;
  /** nerdctl command */
  nerdctlCommand?: string;
}

