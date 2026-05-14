import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { gzipSync } from 'node:zlib'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const projectRoot = path.resolve(__dirname, '..')

const targetDir = path.join(projectRoot, 'public', 'tessdata')
const targetCoreDir = path.join(projectRoot, 'public', 'tesseract-core')
const sourceDir = process.env.TESSDATA_DIR
  ? path.resolve(process.env.TESSDATA_DIR)
  : path.join(projectRoot, 'tessdata')
const tesseractCoreSourceDir = path.join(projectRoot, 'node_modules', 'tesseract.js-core')

const requiredFiles = [
  'chi_sim.traineddata.gz',
  'eng.traineddata.gz',
  'chi_sim_fast.traineddata.gz',
  'chi_sim_best.traineddata.gz',
  'eng_best.traineddata.gz',
]

const downloadSpec = {
  'chi_sim.traineddata.gz': {
    url: 'https://raw.githubusercontent.com/tesseract-ocr/tessdata/main/chi_sim.traineddata',
  },
  'eng.traineddata.gz': {
    url: 'https://raw.githubusercontent.com/tesseract-ocr/tessdata/main/eng.traineddata',
  },
  'chi_sim_fast.traineddata.gz': {
    url: 'https://raw.githubusercontent.com/tesseract-ocr/tessdata_fast/main/chi_sim.traineddata',
  },
  'chi_sim_best.traineddata.gz': {
    url: 'https://raw.githubusercontent.com/tesseract-ocr/tessdata_best/main/chi_sim.traineddata',
  },
  'eng_best.traineddata.gz': {
    url: 'https://raw.githubusercontent.com/tesseract-ocr/tessdata_best/main/eng.traineddata',
  },
}

const requiredCoreFiles = [
  'tesseract-core-lstm.wasm.js',
  'tesseract-core-lstm.wasm',
  'tesseract-core.wasm.js',
  'tesseract-core.wasm',
]

async function exists(filePath) {
  try {
    await fs.access(filePath)
    return true
  } catch {
    return false
  }
}

async function ensureDir(dirPath) {
  await fs.mkdir(dirPath, { recursive: true })
}

async function copyIfNeeded(fileName) {
  const targetPath = path.join(targetDir, fileName)
  if (await exists(targetPath)) {
    return { status: 'exists', fileName, targetPath }
  }

  const sourcePath = path.join(sourceDir, fileName)
  if (!(await exists(sourcePath))) {
    return { status: 'missing', fileName, sourcePath, targetPath }
  }

  await fs.copyFile(sourcePath, targetPath)
  return { status: 'copied', fileName, sourcePath, targetPath }
}

async function copyCoreFileIfNeeded(fileName) {
  const sourcePath = path.join(tesseractCoreSourceDir, fileName)
  const targetPath = path.join(targetCoreDir, fileName)

  if (!(await exists(sourcePath))) {
    throw new Error(`缺少 tesseract-core 文件: ${sourcePath}`)
  }
  if (await exists(targetPath)) {
    return { status: 'exists', fileName, targetPath }
  }

  await fs.copyFile(sourcePath, targetPath)
  return { status: 'copied', fileName, sourcePath, targetPath }
}

async function downloadAndPack(fileName) {
  const spec = downloadSpec[fileName]
  if (!spec) {
    throw new Error(`缺少下载配置: ${fileName}`)
  }

  const response = await fetch(spec.url)
  if (!response.ok) {
    throw new Error(`下载失败(${response.status}): ${spec.url}`)
  }

  const arrayBuffer = await response.arrayBuffer()
  const rawBytes = Buffer.from(arrayBuffer)
  const gzBytes = gzipSync(rawBytes, { level: 9 })
  const targetPath = path.join(targetDir, fileName)
  await fs.writeFile(targetPath, gzBytes)

  return { status: 'downloaded', fileName, targetPath, from: spec.url }
}

async function main() {
  await ensureDir(targetDir)
  await ensureDir(targetCoreDir)

  const coreResults = []
  for (const fileName of requiredCoreFiles) {
    coreResults.push(await copyCoreFileIfNeeded(fileName))
  }

  const results = []
  for (const fileName of requiredFiles) {
    results.push(await copyIfNeeded(fileName))
  }

  const missing = results.filter(item => item.status === 'missing')
  if (missing.length > 0) {
    console.log('\n[prepare-tessdata] 检测到本地缺失，开始自动下载语言包...')
    for (const item of missing) {
      try {
        const downloaded = await downloadAndPack(item.fileName)
        results.push(downloaded)
      } catch (err) {
        console.error(`\n[prepare-tessdata] 下载失败: ${item.fileName}`)
        console.error(`- 本地来源: ${item.sourcePath}`)
        console.error(`- 错误原因: ${err.message || err}`)
        console.error('\n请执行以下任一方式后重试：')
        console.error(`1) 手动将文件放入默认目录: ${sourceDir}`)
        console.error('2) 或设置环境变量 TESSDATA_DIR 指向语言包目录再执行 npm run build')
        process.exit(1)
      }
    }
  }

  for (const item of results) {
    if (item.status === 'copied') {
      console.log(`[prepare-tessdata] copied: ${item.fileName}`)
    } else if (item.status === 'exists') {
      console.log(`[prepare-tessdata] exists: ${item.fileName}`)
    } else if (item.status === 'downloaded') {
      console.log(`[prepare-tessdata] downloaded: ${item.fileName}`)
    }
  }

  for (const item of coreResults) {
    if (item.status === 'copied') {
      console.log(`[prepare-tessdata] core copied: ${item.fileName}`)
    } else if (item.status === 'exists') {
      console.log(`[prepare-tessdata] core exists: ${item.fileName}`)
    }
  }
}

main().catch(err => {
  console.error('[prepare-tessdata] 执行失败:', err)
  process.exit(1)
})
