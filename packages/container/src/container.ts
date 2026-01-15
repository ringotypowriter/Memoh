/**
 * High-level container management API
 */

import { NerdctlClient } from './nerdctl'
import type {
  ContainerConfig,
  ContainerInfo,
  ContainerOperations,
  ExecResult,
  ContainerStats,
  ContainerdOptions,
} from './types'

/**
 * Create a new container
 * 
 * @param config - Container configuration
 * @param options - Containerd client options
 * @returns Container information including ID and metadata
 * 
 * @example
 * ```typescript
 * const container = await createContainer({
 *   name: 'my-app',
 *   image: 'docker.io/library/nginx:latest',
 *   env: { PORT: '8080' },
 * });
 * 
 * console.log('Container created:', container.id);
 * ```
 */
export async function createContainer(
  config: ContainerConfig,
  options?: ContainerdOptions
): Promise<ContainerInfo> {
  const client = new NerdctlClient(options)
  
  // Ensure image is pulled
  await client.pullImage(config.image)
  
  // Create container
  const containerInfo = await client.createContainer(config)
  
  return containerInfo
}

/**
 * Get container operations for an existing container
 * 
 * @param containerIdOrName - Container ID or name
 * @param options - Containerd client options
 * @returns Object with methods to operate on the container
 * 
 * @example
 * ```typescript
 * const container = useContainer('my-app');
 * 
 * // Start the container
 * await container.start();
 * 
 * // Get container info
 * const info = await container.info();
 * console.log('Status:', info.status);
 * 
 * // Execute command
 * const result = await container.exec(['echo', 'hello']);
 * console.log(result.stdout);
 * 
 * // Stop and remove
 * await container.stop();
 * await container.remove();
 * ```
 */
export function useContainer(
  containerIdOrName: string,
  options?: ContainerdOptions
): ContainerOperations {
  const client = new NerdctlClient(options)
  const containerName = containerIdOrName
  
  return {
    /**
     * Start the container
     */
    async start(): Promise<void> {
      await client.startContainer(containerName)
    },
    
    /**
     * Stop the container
     * @param timeout - Graceful shutdown timeout in seconds (default: 10)
     */
    async stop(timeout: number = 10): Promise<void> {
      await client.stopContainer(containerName, timeout)
    },
    
    /**
     * Restart the container
     * @param timeout - Graceful shutdown timeout in seconds (default: 10)
     */
    async restart(timeout: number = 10): Promise<void> {
      await client.stopContainer(containerName, timeout)
      await client.startContainer(containerName)
    },
    
    /**
     * Pause the container
     */
    async pause(): Promise<void> {
      await client.pauseContainer(containerName)
    },
    
    /**
     * Resume a paused container
     */
    async resume(): Promise<void> {
      await client.resumeContainer(containerName)
    },
    
    /**
     * Remove the container
     * @param force - Force remove even if running (default: false)
     */
    async remove(force: boolean = false): Promise<void> {
      await client.removeContainer(containerName, force)
    },
    
    /**
     * Execute a command in the container
     * @param command - Command and arguments to execute
     * @returns Execution result with exit code and output
     */
    async exec(command: string[]): Promise<ExecResult> {
      const result = await client.execInContainer(containerName, command)
      return {
        exitCode: result.exitCode,
        stdout: result.stdout,
        stderr: result.stderr,
      }
    },
    
    /**
     * Get container information
     */
    async info(): Promise<ContainerInfo> {
      return await client.getContainerInfo(containerName)
    },
    
    /**
     * Get container logs
     * @param follow - Follow log output (not implemented yet)
     */
    async logs(follow: boolean = false): Promise<string> {
      if (follow) {
        throw new Error('Follow mode not implemented yet')
      }
      return await client.getContainerLogs(containerName)
    },
    
    /**
     * Get container stats
     * Note: This is a placeholder implementation
     * Real implementation would require parsing nerdctl metrics
     */ 
    async stats(): Promise<ContainerStats> {
      // This is a simplified implementation
      // Full implementation would require parsing nerdctl metrics output
      return {
        cpuUsage: 0,
        memoryUsage: 0,
        memoryLimit: 0,
        networkIO: {
          rxBytes: 0,
          txBytes: 0,
        },
      }
    },

    buildExecCommand(command: string[]): string[] {
      // nerdctl exec with -i to keep STDIN open for MCP servers
      return [...client.nerdctlCommand, 'exec', '-i', containerName, ...command]
    }
  }
}

/**
 * List all containers in the namespace
 * 
 * @param options - Containerd client options
 * @returns Array of container information
 * 
 * @example
 * ```typescript
 * const containers = await listContainers();
 * for (const container of containers) {
 *   console.log(`${container.name}: ${container.status}`);
 * }
 * ```
 */
export async function listContainers(options?: ContainerdOptions): Promise<ContainerInfo[]> {
  const client = new NerdctlClient(options)
  return await client.listContainers()
}

/**
 * Check if a container exists
 * 
 * @param containerIdOrName - Container ID or name
 * @param options - Containerd client options
 * @returns True if container exists, false otherwise
 * 
 * @example
 * ```typescript
 * if (await containerExists('my-app')) {
 *   console.log('Container exists');
 * }
 * ```
 */
export async function containerExists(
  containerIdOrName: string,
  options?: ContainerdOptions
): Promise<boolean> {
  const client = new NerdctlClient(options)
  return await client.containerExists(containerIdOrName)
}

/**
 * Remove all containers in the namespace
 * 
 * @param force - Force remove even if running
 * @param options - Containerd client options
 * 
 * @example
 * ```typescript
 * await removeAllContainers(true);
 * console.log('All containers removed');
 * ```
 */
export async function removeAllContainers(
  force: boolean = false,
  options?: ContainerdOptions
): Promise<void> {
  const client = new NerdctlClient(options)
  const containers = await client.listContainers()
  
  for (const container of containers) {
    await client.removeContainer(container.name, force)
  }
}

