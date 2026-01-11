import type { Command } from 'commander'
import chalk from 'chalk'
import inquirer from 'inquirer'
import ora from 'ora'
import { table } from 'table'
import * as platformCore from '../../core/platform'
import { formatError } from '../../utils'
import { getApiUrl } from '../../core/client'

export function platformCommands(program: Command) {
  program
    .command('list')
    .description('List all platform configurations')
    .action(async () => {
      const spinner = ora('Fetching platform list...').start()
      try {
        const platforms = await platformCore.listPlatforms()
        spinner.succeed(chalk.green('Platform List'))

        if (platforms.length === 0) {
          console.log(chalk.yellow('No platform configurations found'))
          return
        }

        const tableData = [
          ['ID', 'Name', 'Endpoint', 'Active', 'Created'],
          ...platforms.map((item) => [
            item.id.substring(0, 8) + '...',
            item.name,
            item.endpoint,
            item.active ? chalk.green('✓ Active') : chalk.dim('✗ Inactive'),
            new Date(item.createdAt).toLocaleDateString(),
          ]),
        ]

        console.log(table(tableData))
      } catch (error) {
        spinner.fail(chalk.red('Operation failed'))
        if (error instanceof Error) {
          if (error.name === 'AbortError' || error.name === 'TimeoutError') {
            console.error(chalk.red('Connection timeout, please check:'))
            console.error(chalk.yellow('  1. Is the API server running?'))
            console.error(chalk.yellow('  2. Is the API URL correct?'))
            console.error(chalk.dim(`     Current config: ${getApiUrl()}`))
          } else {
            console.error(chalk.red('Error:'), error.message)
          }
        } else {
          console.error(chalk.red('Error:'), String(error))
        }
        process.exit(1)
      }
    })

  program
    .command('create')
    .description('Create platform configuration')
    .option('-n, --name <name>', 'Platform name')
    .option('-e, --endpoint <endpoint>', 'Platform endpoint URL')
    .option('-c, --config <config>', 'Platform config (JSON string)')
    .option('-a, --active', 'Set platform as active', true)
    .action(async (options) => {
      try {
        let { name, endpoint, config, active } = options

        if (!name || !endpoint) {
          const answers = await inquirer.prompt([
            {
              type: 'input',
              name: 'name',
              message: 'Platform name:',
              when: !name,
              validate: (value: string) => {
                if (value.trim()) return true
                return 'Platform name is required'
              },
            },
            {
              type: 'input',
              name: 'endpoint',
              message: 'Platform endpoint URL:',
              when: !endpoint,
              validate: (value: string) => {
                if (value.trim()) return true
                return 'Endpoint is required'
              },
            },
            {
              type: 'input',
              name: 'config',
              message: 'Platform config (JSON string):',
              default: '{}',
              when: !config,
            },
            {
              type: 'confirm',
              name: 'active',
              message: 'Set as active?',
              default: true,
              when: active === undefined,
            },
          ])

          name = name || answers.name
          endpoint = endpoint || answers.endpoint
          config = config || answers.config
          active = active ?? answers.active
        }

        // Parse config JSON
        let configObj: Record<string, unknown> = {}
        if (config) {
          try {
            configObj = JSON.parse(config)
          } catch {
            console.error(chalk.red('Invalid JSON config'))
            process.exit(1)
          }
        }

        const spinner = ora('Creating platform configuration...').start()

        const platform = await platformCore.createPlatform({
          name,
          endpoint,
          config: configObj,
          active,
        })

        spinner.succeed(chalk.green('Platform configuration created successfully'))
        console.log(chalk.blue(`Name: ${platform.name}`))
        console.log(chalk.blue(`Endpoint: ${platform.endpoint}`))
        console.log(chalk.blue(`Active: ${platform.active ? 'Yes' : 'No'}`))
        console.log(chalk.blue(`ID: ${platform.id}`))
      } catch (error) {
        console.error(chalk.red(formatError(error)))
        process.exit(1)
      }
    })

  program
    .command('get <id>')
    .description('Get platform configuration details')
    .action(async (id) => {
      const spinner = ora('Fetching platform configuration...').start()
      try {
        const platform = await platformCore.getPlatform(id)
        spinner.succeed(chalk.green('Platform Configuration'))
        console.log(chalk.blue(`ID: ${platform.id}`))
        console.log(chalk.blue(`Name: ${platform.name}`))
        console.log(chalk.blue(`Endpoint: ${platform.endpoint}`))
        console.log(chalk.blue(`Active: ${platform.active ? 'Yes' : 'No'}`))
        console.log(chalk.blue(`Config: ${JSON.stringify(platform.config, null, 2)}`))
        console.log(chalk.blue(`Created At: ${new Date(platform.createdAt).toLocaleString('en-US')}`))
        console.log(chalk.blue(`Updated At: ${new Date(platform.updatedAt).toLocaleString('en-US')}`))
      } catch (error) {
        spinner.fail(chalk.red('Operation failed'))
        console.error(chalk.red(formatError(error)))
        process.exit(1)
      }
    })

  program
    .command('update <id>')
    .description('Update platform configuration')
    .option('-n, --name <name>', 'Platform name')
    .option('-e, --endpoint <endpoint>', 'Platform endpoint URL')
    .option('-c, --config <config>', 'Platform config (JSON string)')
    .option('-a, --active <active>', 'Set active status (true/false)')
    .action(async (id, options) => {
      try {
        const updates: Record<string, unknown> = {}

        if (options.name) updates.name = options.name
        if (options.endpoint) updates.endpoint = options.endpoint
        if (options.config) {
          try {
            updates.config = JSON.parse(options.config)
          } catch {
            console.error(chalk.red('Invalid JSON config'))
            process.exit(1)
          }
        }
        if (options.active !== undefined) {
          updates.active = options.active === 'true'
        }

        if (Object.keys(updates).length === 0) {
          console.log(chalk.yellow('No updates specified'))
          return
        }

        const spinner = ora('Updating platform configuration...').start()
        const platform = await platformCore.updatePlatform(id, updates as any)
        spinner.succeed(chalk.green('Platform configuration updated successfully'))
        console.log(chalk.blue(`Name: ${platform.name}`))
        console.log(chalk.blue(`Endpoint: ${platform.endpoint}`))
        console.log(chalk.blue(`Active: ${platform.active ? 'Yes' : 'No'}`))
      } catch (error) {
        console.error(chalk.red(formatError(error)))
        process.exit(1)
      }
    })

  program
    .command('update-config <id>')
    .description('Update platform config only')
    .requiredOption('-c, --config <config>', 'Platform config (JSON string)')
    .action(async (id, options) => {
      try {
        let configObj: Record<string, unknown>
        try {
          configObj = JSON.parse(options.config)
        } catch {
          console.error(chalk.red('Invalid JSON config'))
          process.exit(1)
        }

        const spinner = ora('Updating platform config...').start()
        const platform = await platformCore.updatePlatformConfig(id, configObj)
        spinner.succeed(chalk.green('Platform config updated successfully'))
        console.log(chalk.blue(`Name: ${platform.name}`))
        console.log(chalk.blue(`Config: ${JSON.stringify(platform.config, null, 2)}`))
      } catch (error) {
        console.error(chalk.red(formatError(error)))
        process.exit(1)
      }
    })

  program
    .command('delete <id>')
    .description('Delete platform configuration')
    .action(async (id) => {
      try {
        const { confirm } = await inquirer.prompt([
          {
            type: 'confirm',
            name: 'confirm',
            message: chalk.yellow(`Are you sure you want to delete platform configuration ${id}?`),
            default: false,
          },
        ])

        if (!confirm) {
          console.log(chalk.yellow('Cancelled'))
          return
        }

        const spinner = ora('Deleting platform configuration...').start()
        await platformCore.deletePlatform(id)
        spinner.succeed(chalk.green('Platform configuration deleted'))
      } catch (error) {
        console.error(chalk.red(formatError(error)))
        process.exit(1)
      }
    })

  program
    .command('activate <id>')
    .description('Activate platform (admin only)')
    .action(async (id) => {
      const spinner = ora('Activating platform...').start()
      try {
        const platform = await platformCore.activatePlatform(id)
        spinner.succeed(chalk.green('Platform activated successfully'))
        console.log(chalk.blue(`Name: ${platform.name}`))
        console.log(chalk.blue(`Endpoint: ${platform.endpoint}`))
        console.log(chalk.blue(`Active: ${platform.active ? chalk.green('Yes') : 'No'}`))
      } catch (error) {
        spinner.fail(chalk.red('Operation failed'))
        console.error(chalk.red(formatError(error)))
        process.exit(1)
      }
    })

  program
    .command('deactivate <id>')
    .description('Deactivate platform (admin only)')
    .action(async (id) => {
      const spinner = ora('Deactivating platform...').start()
      try {
        const platform = await platformCore.inactivatePlatform(id)
        spinner.succeed(chalk.green('Platform deactivated successfully'))
        console.log(chalk.blue(`Name: ${platform.name}`))
        console.log(chalk.blue(`Endpoint: ${platform.endpoint}`))
        console.log(chalk.blue(`Active: ${platform.active ? 'Yes' : chalk.dim('No')}`))
      } catch (error) {
        spinner.fail(chalk.red('Operation failed'))
        console.error(chalk.red(formatError(error)))
        process.exit(1)
      }
    })
}

