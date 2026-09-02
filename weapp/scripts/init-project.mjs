#!/usr/bin/env node

import { access, readFile, writeFile } from 'node:fs/promises'
import path from 'node:path'
import process from 'node:process'
import { fileURLToPath } from 'node:url'

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const booleanOptions = new Set(['dry-run', 'force', 'help'])
const valueOptions = new Set(['api', 'appid', 'author', 'name', 'storage-prefix', 'title'])

function printHelp() {
  console.log(`
Usage:
  pnpm init:project -- --name <package-name> [options]

Options:
  --title <display-name>         小程序展示名称，默认使用 package name
  --appid <appid>                微信 AppID，默认 touristappid
  --api <base-url>               API 基础地址，默认 https://api.example.com
  --storage-prefix <prefix>      本地存储前缀，默认使用 package name
  --author <author>              新项目 package.json author
  --force                        覆盖已有 .env.local 和 project.private.config.json
  --dry-run                      仅显示将执行的操作
  --help                         显示帮助
`.trim())
}

function parseArguments(argv) {
  const options = {}

  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index]
    if (!argument.startsWith('--')) {
      throw new Error(`无法识别参数：${argument}`)
    }

    const key = argument.slice(2)
    if (booleanOptions.has(key)) {
      options[key] = true
      continue
    }

    if (!valueOptions.has(key)) {
      throw new Error(`未知选项：--${key}`)
    }

    const value = argv[index + 1]
    if (!value || value.startsWith('--')) {
      throw new Error(`--${key} 缺少值`)
    }

    options[key] = value
    index += 1
  }

  return options
}

function validateSingleLine(name, value) {
  if (/\r|\n/.test(value)) {
    throw new Error(`${name} 不能包含换行符`)
  }

  return value.trim()
}

function validateOptions(options) {
  const name = validateSingleLine('name', options.name ?? '')
  if (!name) {
    throw new Error('必须提供 --name')
  }

  if (!/^(?:@[a-z0-9][a-z0-9._-]*\/)?[a-z0-9][a-z0-9._-]*$/.test(name)) {
    throw new Error(`无效的 package name：${name}`)
  }

  const title = validateSingleLine('title', options.title ?? name)
  if (!title) {
    throw new Error('title 不能为空')
  }

  const appid = validateSingleLine('appid', options.appid ?? 'touristappid')
  if (!/^(?:touristappid|wx[0-9a-f]{16})$/i.test(appid)) {
    throw new Error(`无效的微信 AppID：${appid}`)
  }

  const apiBaseUrl = validateSingleLine('api', options.api ?? 'https://api.example.com')
  const apiUrl = new URL(apiBaseUrl)
  if (!['http:', 'https:'].includes(apiUrl.protocol)) {
    throw new Error('API 地址必须使用 http 或 https')
  }
  if (apiUrl.username || apiUrl.password || apiUrl.search || apiUrl.hash) {
    throw new Error('API 地址不能包含凭据、查询参数或 hash')
  }

  const storagePrefix = validateSingleLine(
    'storage-prefix',
    options['storage-prefix'] ?? name.replace(/^@/, '').replace('/', '.'),
  )
  if (!/^[a-z0-9][\w.-]*$/i.test(storagePrefix)) {
    throw new Error(`无效的 storage prefix：${storagePrefix}`)
  }

  const author = options.author
    ? validateSingleLine('author', options.author)
    : undefined
  if (options.author && !author) {
    throw new Error('author 不能为空')
  }

  return {
    name,
    title,
    appid,
    apiBaseUrl: apiUrl.toString().replace(/\/+$/, ''),
    storagePrefix,
    author,
    dryRun: Boolean(options['dry-run']),
    force: Boolean(options.force),
  }
}

async function readJson(relativePath) {
  const content = await readFile(path.join(rootDir, relativePath), 'utf8')
  return JSON.parse(content)
}

async function fileExists(relativePath) {
  try {
    await access(path.join(rootDir, relativePath))
    return true
  }
  catch {
    return false
  }
}

function toJson(value) {
  return `${JSON.stringify(value, null, 2)}\n`
}

async function main() {
  const rawOptions = parseArguments(process.argv.slice(2))
  if (rawOptions.help) {
    printHelp()
    return
  }

  const options = validateOptions(rawOptions)
  const packageJson = await readJson('package.json')
  const projectConfig = await readJson('project.config.json')
  const privateConfig = await readJson('project.private.config.example.json')

  packageJson.name = options.name
  packageJson.version = '0.1.0'
  packageJson.description = `${options.title} 微信小程序`
  if (options.author) {
    packageJson.author = options.author
  }
  else {
    delete packageJson.author
  }
  delete packageJson.repository
  delete packageJson.bugs

  projectConfig.appid = options.appid
  projectConfig.projectname = options.name
  projectConfig.description = `${options.title}项目配置文件`

  privateConfig.projectname = options.name

  const envContent = [
    `VITE_APP_NAME=${JSON.stringify(options.title)}`,
    `VITE_STORAGE_PREFIX=${options.storagePrefix}`,
    `VITE_API_BASE_URL=${options.apiBaseUrl}`,
    'VITE_API_TIMEOUT=10000',
    'VITE_ENABLE_LOG=true',
    '',
  ].join('\n')

  const writes = [
    { path: 'package.json', content: toJson(packageJson), guarded: false },
    { path: 'project.config.json', content: toJson(projectConfig), guarded: false },
    { path: '.env.local', content: envContent, guarded: true },
    {
      path: 'project.private.config.json',
      content: toJson(privateConfig),
      guarded: true,
    },
  ]

  for (const item of writes) {
    const exists = await fileExists(item.path)
    if (item.guarded && exists && !options.force) {
      console.log(`跳过 ${item.path}（文件已存在，使用 --force 可覆盖）`)
      continue
    }

    console.log(`${options.dryRun ? '计划写入' : '已写入'} ${item.path}`)
    if (!options.dryRun) {
      await writeFile(path.join(rootDir, item.path), item.content, 'utf8')
    }
  }

  if (options.dryRun) {
    console.log('dry-run 完成，未修改文件。')
    return
  }

  console.log('\n项目初始化完成。接下来执行：pnpm install && pnpm check')
}

main().catch((error) => {
  console.error(error instanceof Error ? error.message : error)
  process.exitCode = 1
})
