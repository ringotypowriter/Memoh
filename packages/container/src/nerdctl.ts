/**
 * Nerdctl client implementation (Docker-compatible CLI for containerd)
 */

import { execa } from 'execa'
import type { ContainerConfig, ContainerInfo, ContainerStatus, ContainerdOptions } from './types'

/**
 * Nerdctl client for managing containers
 * Provides a Docker-like interface for containerd
 */
export class NerdctlClient {
  private namespace: string
  private socket?: string
  private timeout: number
  nerdctlCommand: string[]

  constructor(options: ContainerdOptions = {}) {
    this.namespace = options.namespace || 'default'
    this.socket = options.socket || process.env.CONTAINERD_SOCKET
    this.timeout = options.timeout || 30000
    // Support commands like "lima nerdctl"
    const rawCommand = options.ctrCommand || process.env.CTR_COMMAND || 'nerdctl'
    this.nerdctlCommand = rawCommand.split(' ').filter(part => part.length > 0)
  }

  /**
   * Build nerdctl command with global options
   */
  private buildCommand(args: string[]): string[] {
    // Split command to support "lima nerdctl"
    const cmd = [...this.nerdctlCommand]
    
    // Add global options before the subcommand
    if (this.socket) {
      cmd.push('--address', this.socket)
    }
    
    cmd.push('--namespace', this.namespace)
    cmd.push(...args)
    
    return cmd
  }

  /**
   * Execute nerdctl command
   */
  private async exec(args: string[]): Promise<{ stdout: string; stderr: string }> {
    const cmd = this.buildCommand(args)
    const [program, ...programArgs] = cmd
    
    try {
      const result = await execa(program, programArgs, {
        timeout: this.timeout,
      })
      
      return {
        stdout: result.stdout,
        stderr: result.stderr,
      }
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : String(error)
      throw new Error(`Nerdctl command failed: ${message}`)
    }
  }

  /**
   * Pull container image
   */
  async pullImage(image: string): Promise<void> {
    await this.exec(['pull', image])
  }

  /**
   * Create a new container
   */
  async createContainer(config: ContainerConfig): Promise<ContainerInfo> {
    const args = ['container', 'create']
    
    // Add container name
    args.push('--name', config.name)
    
    // Add environment variables
    if (config.env) {
      for (const [key, value] of Object.entries(config.env)) {
        args.push('--env', `${key}=${value}`)
      }
    }
    
    // Add working directory
    if (config.workingDir) {
      args.push('--workdir', config.workingDir)
    }
    
    // Add labels
    if (config.labels) {
      for (const [key, value] of Object.entries(config.labels)) {
        args.push('--label', `${key}=${value}`)
      }
    }
    
    // Add mounts (nerdctl uses Docker-style mount syntax)
    if (config.mounts && config.mounts.length > 0) {
      for (const mount of config.mounts) {
        let mountStr = `type=${mount.type},src=${mount.source},dst=${mount.target}`
        if (mount.readonly) {
          mountStr += ',readonly'
        }
        args.push('--mount', mountStr)
      }
    }
    
    // Add image
    args.push(config.image)
    
    // Add command if specified
    if (config.command && config.command.length > 0) {
      args.push(...config.command)
    }
    
    await this.exec(args)
    
    // Return container info
    return this.getContainerInfo(config.name)
  }

  /**
   * Start a container
   */
  async startContainer(name: string): Promise<void> {
    // Check container status and handle accordingly
    try {
      const status = await this.getContainerStatus(name)
      
      if (status === 'running') {
        console.log(`Container ${name} is already running`)
        return
      }
      
      if (status === 'paused') {
        console.log(`Container ${name} is paused, unpausing first...`)
        await this.exec(['unpause', name])
        return
      }
      
      // For 'created' or 'stopped' status, we can start
    } catch {
      // Container might not exist, let start command handle it
    }
    
    await this.exec(['start', name])
  }

  /**
   * Stop a container
   */
  async stopContainer(name: string, timeout: number = 10): Promise<void> {
    // Check if container is running
    try {
      const status = await this.getContainerStatus(name)
      if (status !== 'running') {
        console.log(`Container ${name} is not running (status: ${status})`)
        return
      }
    } catch {
      // Container might not exist, let stop command handle it
    }
    
    await this.exec(['stop', '--time', timeout.toString(), name])
  }

  /**
   * Pause a container
   */
  async pauseContainer(name: string): Promise<void> {
    // Check if container is running before pausing
    const status = await this.getContainerStatus(name)
    if (status !== 'running') {
      console.log(`Container ${name} cannot be paused (status: ${status})`)
      return
    }
    
    await this.exec(['pause', name])
  }

  /**
   * Resume a paused container
   */
  async resumeContainer(name: string): Promise<void> {
    // Check if container is paused before resuming
    const status = await this.getContainerStatus(name)
    if (status !== 'paused') {
      console.log(`Container ${name} is not paused (status: ${status})`)
      return
    }
    
    await this.exec(['unpause', name])
  }

  /**
   * Remove a container
   */
  async removeContainer(name: string, force: boolean = false): Promise<void> {
    const args = ['rm']
    if (force) {
      args.push('--force')
    }
    args.push(name)
    await this.exec(args)
  }

  /**
   * Execute command in container
   */
  async execInContainer(name: string, command: string[]): Promise<{ stdout: string; stderr: string; exitCode: number }> {
    const args = ['exec', name, ...command]
    
    try {
      const result = await this.exec(args)
      return {
        stdout: result.stdout,
        stderr: result.stderr,
        exitCode: 0,
      }
    } catch (error: unknown) {
      const err = error as { stdout?: string; stderr?: string; exitCode?: number; message?: string }
      return {
        stdout: err.stdout || '',
        stderr: err.stderr || err.message || '',
        exitCode: err.exitCode || 1,
      }
    }
  }

  /**
   * Get container information
   */
  async getContainerInfo(name: string): Promise<ContainerInfo> {
    const result = await this.exec(['inspect', name])
    
    try {
      const data = JSON.parse(result.stdout)
      const info = Array.isArray(data) ? data[0] : data
      
      // Parse nerdctl inspect output (similar to Docker)
      return {
        id: info.Id || name,
        name: info.Name?.replace(/^\//, '') || name,
        image: info.Config?.Image || info.Image || '',
        status: this.parseStatus(info.State),
        namespace: this.namespace,
        createdAt: info.Created ? new Date(info.Created) : new Date(),
        labels: info.Config?.Labels || {},
      }
    } catch {
      // Fallback if JSON parsing fails
      return {
        id: name,
        name: name,
        image: '',
        status: 'unknown',
        namespace: this.namespace,
        createdAt: new Date(),
      }
    }
  }

  /**
   * Parse container status from inspect output
   */
  private parseStatus(state: unknown): ContainerStatus {
    const s = state as { Running?: boolean; Paused?: boolean; Status?: string; Dead?: boolean }
    if (!s) return 'unknown'
    
    if (s.Running) return 'running'
    if (s.Paused) return 'paused'
    if (s.Status === 'created') return 'created'
    if (s.Status === 'exited' || s.Dead) return 'stopped'
    
    return 'unknown'
  }

  /**
   * Get container status
   */
  async getContainerStatus(name: string): Promise<ContainerStatus> {
    try {
      const info = await this.getContainerInfo(name)
      return info.status
    } catch {
      return 'unknown'
    }
  }

  /**
   * Get container logs
   */
  async getContainerLogs(name: string): Promise<string> {
    try {
      const result = await this.exec(['logs', name])
      return result.stdout
    } catch (error: unknown) {
      return error instanceof Error ? error.message : ''
    }
  }

  /**
   * List all containers
   */
  async listContainers(): Promise<ContainerInfo[]> {
    const result = await this.exec(['ps', '--all', '--format', '{{.Names}}'])
    const containerNames = result.stdout.split('\n').filter(name => name.trim())
    
    const containers: ContainerInfo[] = []
    for (const name of containerNames) {
      try {
        const info = await this.getContainerInfo(name)
        containers.push(info)
      } catch {
        // Skip containers that can't be accessed
      }
    }
    
    return containers
  }

  /**
   * Check if container exists
   */
  async containerExists(name: string): Promise<boolean> {
    try {
      await this.getContainerInfo(name)
      return true
    } catch {
      return false
    }
  }
}

