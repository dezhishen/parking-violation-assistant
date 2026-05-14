import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const root = path.resolve(__dirname, '..')

const requiredFiles = [
  'frontend/dist/index.html',
  'frontend/dist/assets',
]

async function exists(p) {
  try {
    await fs.access(p)
    return true
  } catch {
    return false
  }
}

async function verifyAssetsDir() {
  const assetsDir = path.join(root, 'frontend/dist/assets')
  const entries = await fs.readdir(assetsDir)
  const hasJs = entries.some(name => name.endsWith('.js'))
  const hasCss = entries.some(name => name.endsWith('.css'))
  if (!hasJs || !hasCss) {
    throw new Error('frontend/dist/assets 缺少 js 或 css 构建产物')
  }
}

async function main() {
  const missing = []
  for (const rel of requiredFiles) {
    const abs = path.join(root, rel)
    if (!(await exists(abs))) {
      missing.push(rel)
    }
  }

  if (missing.length > 0) {
    console.error('\n[verify-frontend-dist] 以下文件缺失：')
    for (const item of missing) {
      console.error(`- ${item}`)
    }
    console.error('\n请先执行：cd frontend && npm run build')
    process.exit(1)
  }

  await verifyAssetsDir()
  console.log('[verify-frontend-dist] 前端构建产物完整')
}

main().catch((err) => {
  console.error('[verify-frontend-dist] 校验失败:', err.message || err)
  process.exit(1)
})
