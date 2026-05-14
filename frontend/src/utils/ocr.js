import { createWorker, setLogging } from 'tesseract.js'

setLogging(false)

const PROVINCE_CHARS = '京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤青藏川宁琼'
const PLATE_RE = new RegExp(`([${PROVINCE_CHARS}][A-Z][A-Z0-9]{5,6})`)
const TIME_RE = /(20\d{2})[-/年.](\d{1,2})[-/月.](\d{1,2})(?:日)?\s+([01]?\d|2[0-3])[:时](\d{1,2})(?:[:分](\d{1,2}))?/

let workerPromise = null
const TESS_LANG_PATH = '/tessdata'
const TESS_CORE_PATHS = [
  '/tesseract-core/tesseract-core-lstm.wasm.js',
  '/tesseract-core/tesseract-core.wasm.js',
]
const OCR_PROFILE_STORAGE_KEY = 'countcar_ocr_model_profile'
const PROFILE_STANDARD = {
  key: 'standard',
  lang: 'chi_sim+eng',
  files: ['chi_sim.traineddata.gz', 'eng.traineddata.gz']
}
const PROFILE_FAST = {
  key: 'fast',
  lang: 'chi_sim_fast+eng',
  files: ['chi_sim_fast.traineddata.gz', 'eng.traineddata.gz']
}
const PROFILE_BEST = {
  key: 'best',
  lang: 'chi_sim_best+eng_best',
  files: ['chi_sim_best.traineddata.gz', 'eng_best.traineddata.gz']
}

export const OCR_MODEL_OPTIONS = [
  { value: 'standard', label: '标准（默认）' },
  { value: 'fast', label: '快速（精度略低）' },
  { value: 'best', label: '高精度（更慢）' },
]

function normalizeProfileName(name) {
  const value = String(name || '').toLowerCase()
  if (value === 'best' || value === 'fast' || value === 'standard') {
    return value
  }
  return 'standard'
}

function getStoredProfileName() {
  try {
    if (typeof localStorage === 'undefined') {
      return ''
    }
    return localStorage.getItem(OCR_PROFILE_STORAGE_KEY) || ''
  } catch {
    return ''
  }
}

function setStoredProfileName(name) {
  try {
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(OCR_PROFILE_STORAGE_KEY, name)
    }
  } catch {
    // 忽略本地存储不可用场景。
  }
}

function getProfileByName(name) {
  if (name === 'best') return PROFILE_BEST
  if (name === 'fast') return PROFILE_FAST
  return PROFILE_STANDARD
}

const envProfileName = normalizeProfileName(import.meta.env.VITE_OCR_MODEL_PROFILE || 'standard')
let currentProfileName = normalizeProfileName(getStoredProfileName() || envProfileName)

export function getOCRModelProfile() {
  return currentProfileName
}

export async function setOCRModelProfile(nextName) {
  const normalized = normalizeProfileName(nextName)
  if (normalized === currentProfileName) {
    return currentProfileName
  }

  const previousWorkerPromise = workerPromise
  workerPromise = null
  currentProfileName = normalized
  setStoredProfileName(normalized)

  if (previousWorkerPromise) {
    try {
      const oldWorker = await previousWorkerPromise
      await oldWorker.terminate()
    } catch {
      // 旧 worker 释放失败不影响新配置生效。
    }
  }

  return currentProfileName
}

async function ensureLocalTessdata(files) {
  for (const file of files) {
    const url = `${TESS_LANG_PATH}/${file}`
    try {
      const resp = await fetch(url, { method: 'HEAD', cache: 'no-store' })
      if (!resp.ok) {
        throw new Error(`缺少离线语言包: ${file}`)
      }
    } catch (err) {
			throw new Error(`OCR离线模型未就绪，请确认存在 ${url}`)
    }
  }
}

async function createWorkerWithProfile(profile) {
  await ensureLocalTessdata(profile.files)

  let lastError = null
  for (const corePath of TESS_CORE_PATHS) {
    try {
      return await createWorker(profile.lang, 1, {
        langPath: TESS_LANG_PATH,
        corePath,
        cacheMethod: 'none',
        gzip: true,
      })
    } catch (err) {
      lastError = err
    }
  }

  throw lastError || new Error('OCR 内核初始化失败')
}

function getWorker() {
  if (!workerPromise) {
    workerPromise = (async () => {
      const requestedProfile = getProfileByName(currentProfileName)
      try {
        return await createWorkerWithProfile(requestedProfile)
      } catch (err) {
        // fast/best 初始化失败时自动回退 standard，避免整个识别流程崩溃。
        if (requestedProfile.key !== PROFILE_STANDARD.key) {
          currentProfileName = PROFILE_STANDARD.key
          setStoredProfileName(PROFILE_STANDARD.key)
          return await createWorkerWithProfile(PROFILE_STANDARD)
        }
        throw err
      }
    })().catch((err) => {
      // 初始化失败后清空缓存，允许后续重试。
      workerPromise = null
      throw err
    })
  }
  return workerPromise
}

function normalizePlate(raw) {
  if (!raw) return ''
  return raw
    .toUpperCase()
    .replace(/[\s\r\n\t]+/g, '')
    .replace(/[·•.。:：,，;；'"`~!@#$%^&*()_+=\-/?\\|<>\[\]{}]/g, '')
    .replace(/O/g, '0')
}

function sanitizeOCRText(raw) {
  if (!raw) return ''
  return raw
    .toUpperCase()
    .replace(/[\r\n]+/g, ' ')
    .replace(/[•。]/g, '·')
}

function normalizeOCRLines(raw) {
  if (!raw) return []
  return String(raw)
    .replace(/\r\n/g, '\n')
    .replace(/\r/g, '\n')
    .split('\n')
    .map(line => line.trim())
    .filter(line => line.length > 0)
}

function scoreOCRText(raw) {
  if (!raw) return 0
  const matched = String(raw).match(/[\u4e00-\u9fa5A-Z0-9]/gi)
  return matched ? matched.length : 0
}

function pickProvinceChar(text) {
  if (!text) return ''
  for (const ch of text) {
    if (PROVINCE_CHARS.includes(ch)) {
      return ch
    }
  }
  return ''
}

function toTimeString(match) {
  if (!match) return ''
  const year = Number(match[1])
  const month = Number(match[2])
  const day = Number(match[3])
  const hour = Number(match[4])
  const minute = Number(match[5])
  const second = Number(match[6] || 0)

  const date = new Date(year, month - 1, day, hour, minute, second)
  if (
    Number.isNaN(date.getTime()) ||
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day ||
    date.getHours() !== hour ||
    date.getMinutes() !== minute
  ) {
    return ''
  }

  const pad = n => String(n).padStart(2, '0')
  return `${year}-${pad(month)}-${pad(day)} ${pad(hour)}:${pad(minute)}:${pad(second)}`
}

export function extractOCRFields(text) {
  const normalized = normalizePlate(text)
  const plateMatch = normalized.match(PLATE_RE)
  const fallbackPlateMatch = normalized.match(/([A-Z][A-Z0-9]{5,6})/)
  const timeMatch = text?.match(TIME_RE)

  return {
    plate_number: plateMatch ? plateMatch[1] : (fallbackPlateMatch ? fallbackPlateMatch[1] : ''),
    parking_time: toTimeString(timeMatch),
    raw_text: text || ''
  }
}

function hasProvincePrefix(plate) {
  if (!plate) return false
  const first = plate[0]
  return PROVINCE_CHARS.includes(first)
}

function isPlateWithoutProvince(plate) {
  return /^[A-Z][A-Z0-9]{5,6}$/.test(plate || '')
}

function fillProvinceIfMissing(plate, provinceChar) {
  if (!plate || !provinceChar) return plate
  if (hasProvincePrefix(plate)) return plate
  if (!isPlateWithoutProvince(plate)) return plate
  return `${provinceChar}${plate}`
}

function drawRegion(image, x, y, w, h, options = {}) {
  const { scale = 2, mode = 'binary' } = options
  const canvas = document.createElement('canvas')
  canvas.width = Math.max(1, Math.floor(w * scale))
  canvas.height = Math.max(1, Math.floor(h * scale))
  const ctx = canvas.getContext('2d', { willReadFrequently: true })
  ctx.drawImage(image, x, y, w, h, 0, 0, canvas.width, canvas.height)

  if (mode === 'none') {
    return canvas
  }

  // 灰度/二值多路预处理，提高不同光照下的识别稳定性。
  const img = ctx.getImageData(0, 0, canvas.width, canvas.height)
  const d = img.data
  let graySum = 0
  const pixelCount = d.length / 4
  const grayscale = new Float32Array(pixelCount)

  for (let i = 0; i < d.length; i += 4) {
    const r = d[i]
    const g = d[i + 1]
    const b = d[i + 2]
    const gray = 0.299 * r + 0.587 * g + 0.114 * b
    const idx = i / 4
    grayscale[idx] = gray
    graySum += gray
  }

  if (mode === 'gray') {
    for (let i = 0; i < d.length; i += 4) {
      const g = Math.max(0, Math.min(255, Math.round((grayscale[i / 4] - 128) * 1.15 + 128)))
      d[i] = g
      d[i + 1] = g
      d[i + 2] = g
    }
    ctx.putImageData(img, 0, 0)
    return canvas
  }

  const avgGray = graySum / Math.max(1, pixelCount)
  const threshold = Math.max(95, Math.min(185, avgGray * 0.92))
  for (let i = 0; i < d.length; i += 4) {
    const boosted = grayscale[i / 4] > threshold ? 255 : 0
    d[i] = boosted
    d[i + 1] = boosted
    d[i + 2] = boosted
  }
  ctx.putImageData(img, 0, 0)
  return canvas
}

function getImageSize(imageLike) {
  if (!imageLike) {
    return { width: 0, height: 0 }
  }
  const width = imageLike.naturalWidth || imageLike.width || 0
  const height = imageLike.naturalHeight || imageLike.height || 0
  return { width, height }
}

function buildPlateCanvas(image) {
  const { width: w, height: h } = getImageSize(image)
  if (w <= 0 || h <= 0) {
    return null
  }

  // 选区图像经常过小，这里强制放大到稳定输入尺寸再做单行识别。
  const scale = Math.max(1, 320 / w, 80 / h)
  return {
    raw: drawRegion(image, 0, 0, w, h, { scale, mode: 'none' }),
    gray: drawRegion(image, 0, 0, w, h, { scale, mode: 'gray' }),
    binary: drawRegion(image, 0, 0, w, h, { scale, mode: 'binary' }),
  }
}

function loadImage(input) {
  return new Promise((resolve, reject) => {
    if (typeof HTMLCanvasElement !== 'undefined' && input instanceof HTMLCanvasElement) {
      resolve(input)
      return
    }
    if (typeof OffscreenCanvas !== 'undefined' && input instanceof OffscreenCanvas) {
      resolve(input)
      return
    }
    if (typeof ImageBitmap !== 'undefined' && input instanceof ImageBitmap) {
      resolve(input)
      return
    }
    if (input instanceof HTMLImageElement && input.complete) {
      resolve(input)
      return
    }

    const img = new Image()
    img.crossOrigin = 'anonymous'
    img.onload = () => resolve(img)
    img.onerror = reject

    if (input instanceof Blob) {
      const url = URL.createObjectURL(input)
      img.onload = () => {
        URL.revokeObjectURL(url)
        resolve(img)
      }
      img.onerror = (e) => {
        URL.revokeObjectURL(url)
        reject(e)
      }
      img.src = url
      return
    }

    img.src = String(input)
  })
}

async function recognizeOne(worker, imageLike) {
  const size = getImageSize(imageLike)
  if (size.width < 3 || size.height < 3) {
    return ''
  }
  const { data } = await worker.recognize(imageLike)
  return sanitizeOCRText(data?.text || '')
}

async function recognizeRaw(worker, imageLike) {
  const size = getImageSize(imageLike)
  if (size.width < 3 || size.height < 3) {
    return ''
  }
  const { data } = await worker.recognize(imageLike)
  return (data?.text || '').trim()
}

async function recognizePlateOneLine(worker, imageLike) {
  const size = getImageSize(imageLike)
  if (size.width < 3 || size.height < 3) {
    return ''
  }
  await worker.setParameters({
    tessedit_pageseg_mode: '7',
    tessedit_char_whitelist: `${PROVINCE_CHARS}ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789·`,
  })
  const { data } = await worker.recognize(imageLike)
  await worker.setParameters({
    tessedit_pageseg_mode: '6',
  })
  return sanitizeOCRText(data?.text || '')
}

async function recognizeProvinceFromPlate(worker, plateImage) {
  const size = getImageSize(plateImage)
  if (size.width < 3 || size.height < 3) {
    return ''
  }

  const leftRaw = drawRegion(plateImage, 0, 0, size.width * 0.34, size.height, { scale: 4.2, mode: 'none' })
  const leftGray = drawRegion(plateImage, 0, 0, size.width * 0.34, size.height, { scale: 4.2, mode: 'gray' })
  const leftBinary = drawRegion(plateImage, 0, 0, size.width * 0.34, size.height, { scale: 4.2, mode: 'binary' })

  const texts = []
  texts.push(await safeRecognize(() => recognizeOne(worker, leftRaw)))
  texts.push(await safeRecognize(() => recognizeOne(worker, leftGray)))
  texts.push(await safeRecognize(() => recognizeOne(worker, leftBinary)))

  return pickProvinceChar(texts.join(' '))
}

async function safeRecognize(fn) {
  try {
    return await fn()
  } catch {
    return ''
  }
}

export async function recognizeTextLines(imageInput, options = {}) {
  const { plateOnly = false } = options
  const worker = await getWorker()
  const image = await loadImage(imageInput)
  const w = image.naturalWidth || image.width
  const h = image.naturalHeight || image.height

  const fullCanvasRaw = drawRegion(image, 0, 0, w, h, { scale: 1.2, mode: 'none' })
  const fullCanvasGray = drawRegion(image, 0, 0, w, h, { scale: 1.2, mode: 'gray' })
  const fullCanvasBinary = drawRegion(image, 0, 0, w, h, { scale: 1.2, mode: 'binary' })

  const centerLowerCanvasRaw = drawRegion(
    image,
    w * 0.18,
    h * 0.52,
    w * 0.64,
    h * 0.34,
    { scale: 2.6, mode: 'none' }
  )
  const centerLowerCanvasBinary = drawRegion(
    image,
    w * 0.18,
    h * 0.52,
    w * 0.64,
    h * 0.34,
    { scale: 2.6, mode: 'binary' }
  )

  const candidates = []
  if (plateOnly) {
    const plateCanvasSet = buildPlateCanvas(image)
    if (plateCanvasSet) {
      const t1 = await safeRecognize(() => recognizeRaw(worker, plateCanvasSet.raw))
      const t2 = await safeRecognize(() => recognizeRaw(worker, plateCanvasSet.gray))
      const t3 = await safeRecognize(() => recognizeRaw(worker, plateCanvasSet.binary))
      candidates.push(t1, t2, t3)
    }
    const t4 = await safeRecognize(() => recognizeRaw(worker, centerLowerCanvasRaw))
    const t5 = await safeRecognize(() => recognizeRaw(worker, centerLowerCanvasBinary))
    candidates.push(t4, t5)
  } else {
    const t1 = await safeRecognize(() => recognizeRaw(worker, fullCanvasRaw))
    const t2 = await safeRecognize(() => recognizeRaw(worker, fullCanvasGray))
    const t3 = await safeRecognize(() => recognizeRaw(worker, fullCanvasBinary))
    const t4 = await safeRecognize(() => recognizeRaw(worker, centerLowerCanvasRaw))
    const t5 = await safeRecognize(() => recognizeRaw(worker, centerLowerCanvasBinary))
    candidates.push(t1, t2, t3, t4, t5)
  }

  const bestText = candidates.reduce((best, curr) => {
    if (scoreOCRText(curr) > scoreOCRText(best)) {
      return curr
    }
    return best
  }, '')

  const lines = normalizeOCRLines(bestText)
  return {
    lines,
    raw_text: lines.join('\n')
  }
}

export async function recognizeImageWithBuiltinOCR(imageInput, options = {}) {
  const { plateOnly = false } = options
  const worker = await getWorker()
  const image = await loadImage(imageInput)

  const w = image.naturalWidth || image.width
  const h = image.naturalHeight || image.height

  const fullCanvasRaw = drawRegion(image, 0, 0, w, h, { scale: 1.2, mode: 'none' })
  const fullCanvasGray = drawRegion(image, 0, 0, w, h, { scale: 1.2, mode: 'gray' })
  const fullCanvasBinary = drawRegion(image, 0, 0, w, h, { scale: 1.2, mode: 'binary' })
  const centerLowerCanvasRaw = drawRegion(
    image,
    w * 0.18,
    h * 0.52,
    w * 0.64,
    h * 0.34,
    { scale: 2.6, mode: 'none' }
  )
  const centerLowerCanvasBinary = drawRegion(
    image,
    w * 0.18,
    h * 0.52,
    w * 0.64,
    h * 0.34,
    { scale: 2.6, mode: 'binary' }
  )
  const tighterPlateCanvasBinary = drawRegion(
    image,
    w * 0.28,
    h * 0.58,
    w * 0.44,
    h * 0.20,
    { scale: 3.0, mode: 'binary' }
  )
  const tighterPlateCanvasRaw = drawRegion(
    image,
    w * 0.28,
    h * 0.58,
    w * 0.44,
    h * 0.20,
    { scale: 3.0, mode: 'none' }
  )

  const texts = []
  if (plateOnly) {
    const plateCanvasSet = buildPlateCanvas(image)
    if (plateCanvasSet) {
      texts.push(await safeRecognize(() => recognizePlateOneLine(worker, plateCanvasSet.raw)))
      texts.push(await safeRecognize(() => recognizePlateOneLine(worker, plateCanvasSet.gray)))
      texts.push(await safeRecognize(() => recognizePlateOneLine(worker, plateCanvasSet.binary)))
    }
    texts.push(await safeRecognize(() => recognizePlateOneLine(worker, centerLowerCanvasRaw)))
    texts.push(await safeRecognize(() => recognizePlateOneLine(worker, centerLowerCanvasBinary)))
    texts.push(await safeRecognize(() => recognizePlateOneLine(worker, tighterPlateCanvasRaw)))
    texts.push(await safeRecognize(() => recognizePlateOneLine(worker, tighterPlateCanvasBinary)))
  } else {
    texts.push(await safeRecognize(() => recognizeOne(worker, fullCanvasRaw)))
    texts.push(await safeRecognize(() => recognizeOne(worker, fullCanvasGray)))
    texts.push(await safeRecognize(() => recognizeOne(worker, fullCanvasBinary)))
    texts.push(await safeRecognize(() => recognizeOne(worker, centerLowerCanvasRaw)))
    texts.push(await safeRecognize(() => recognizeOne(worker, centerLowerCanvasBinary)))
    texts.push(await safeRecognize(() => recognizePlateOneLine(worker, tighterPlateCanvasRaw)))
    texts.push(await safeRecognize(() => recognizePlateOneLine(worker, tighterPlateCanvasBinary)))
  }

  const mergedText = texts.filter(Boolean).join('\n')
  const extracted = extractOCRFields(mergedText)

  if (!hasProvincePrefix(extracted.plate_number) && isPlateWithoutProvince(extracted.plate_number)) {
    const plateProbe = buildPlateCanvas(image)
    if (plateProbe) {
      const province = await recognizeProvinceFromPlate(worker, plateProbe.raw)
      extracted.plate_number = fillProvinceIfMissing(extracted.plate_number, province)
    }
  }

  return extracted
}
