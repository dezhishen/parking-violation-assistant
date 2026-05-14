#!/usr/bin/env node

import crypto from "node:crypto";
import fs from "node:fs";
import fsp from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { spawn } from "node:child_process";

const PROJECT_ROOT = process.cwd();

const defaultDetModel = {
  url: "https://www.modelscope.cn/models/RapidAI/RapidOCR/resolve/v3.8.0/onnx/PP-OCRv4/det/ch_PP-OCRv4_det_mobile.onnx",
  sha256: "d2a7720d45a54257208b1e13e36a8479894cb74155a5efe29462512d42f49da9",
};

const defaultRecModel = {
  url: "https://www.modelscope.cn/models/RapidAI/RapidOCR/resolve/v3.8.0/onnx/PP-OCRv4/rec/ch_PP-OCRv4_rec_mobile.onnx",
  sha256: "48fc40f24f6d2a207a2b1091d3437eb3cc3eb6b676dc3ef9c37384005483683b",
};

const defaultClsModel = {
  url: "https://www.modelscope.cn/models/RapidAI/RapidOCR/resolve/v3.8.0/onnx/PP-OCRv4/cls/ch_ppocr_mobile_v2.0_cls_mobile.onnx",
  sha256: "e47acedf663230f8863ff1ab0e64dd2d82b838fceb5957146dab185a89d6215c",
};

const defaultKeysFile = {
  url: "https://raw.githubusercontent.com/RapidAI/RapidOcrOnnx/main/models/ppocr_keys_v1.txt",
};

const runnerReleaseTag = process.env.COUNTCAR_OCR_RUNNER_TAG || "1.2.2";
const runnerReleaseBase = `https://github.com/RapidAI/RapidOcrOnnx/releases/download/${runnerReleaseTag}`;

const runnerSpecByPlatform = {
  linux: {
    archiveUrl: `${runnerReleaseBase}/linux-bin.7z`,
    archiveInnerPath: "linux-bin/Linux-BIN-CPU/RapidOcrOnnx",
    outputName: "onnx_ocr_runner",
  },
  darwin: {
    archiveUrl: `${runnerReleaseBase}/macos-bin.7z`,
    archiveInnerPath: "macos-bin/Darwin-BIN-CPU/RapidOcrOnnx",
    outputName: "onnx_ocr_runner",
  },
  windows: {
    archiveUrl: `${runnerReleaseBase}/windows-bin.7z`,
    archiveInnerPath: "windows-bin/win-BIN-CPU-x64/RapidOcrOnnx.exe",
    outputName: "onnx_ocr_runner.exe",
  },
};

function parseArgs(argv) {
  const out = {};
  for (let i = 2; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith("--")) continue;
    const key = token.slice(2);
    const next = argv[i + 1];
    if (next && !next.startsWith("--")) {
      out[key] = next;
      i += 1;
    } else {
      out[key] = "true";
    }
  }
  return out;
}

function mapNodePlatformToTarget(p) {
  if (p === "win32") return "windows";
  if (p === "darwin") return "darwin";
  return "linux";
}

function ensureDir(dir) {
  return fsp.mkdir(dir, { recursive: true });
}

async function fileExists(p) {
  try {
    await fsp.access(p);
    return true;
  } catch {
    return false;
  }
}

function runCommand(command, args, options = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      stdio: "inherit",
      ...options,
    });
    child.on("error", reject);
    child.on("close", (code) => {
      if (code === 0) {
        resolve();
      } else {
        reject(new Error(`${command} exited with code ${code}`));
      }
    });
  });
}

async function sha256OfFile(filePath) {
  const hash = crypto.createHash("sha256");
  const stream = fs.createReadStream(filePath);
  await new Promise((resolve, reject) => {
    stream.on("data", (chunk) => hash.update(chunk));
    stream.on("error", reject);
    stream.on("end", resolve);
  });
  return hash.digest("hex");
}

async function downloadToFile(url, outputPath) {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`下载失败: ${url} (${response.status})`);
  }
  const buffer = Buffer.from(await response.arrayBuffer());
  await fsp.writeFile(outputPath, buffer);
}

async function ensureModel(filePath, { url, sha256 }) {
  if (await fileExists(filePath)) {
    if (sha256) {
      const actual = await sha256OfFile(filePath);
      if (actual.toLowerCase() === sha256.toLowerCase()) {
        console.log(`[prepare-onnx-ocr] model exists: ${path.basename(filePath)}`);
        return;
      }
      console.warn(`[prepare-onnx-ocr] sha mismatch, re-download: ${path.basename(filePath)}`);
    } else {
      console.log(`[prepare-onnx-ocr] model exists: ${path.basename(filePath)}`);
      return;
    }
  }

  console.log(`[prepare-onnx-ocr] downloading model: ${url}`);
  await downloadToFile(url, filePath);
  if (sha256) {
    const actual = await sha256OfFile(filePath);
    if (actual.toLowerCase() !== sha256.toLowerCase()) {
      throw new Error(`模型校验失败: ${filePath}`);
    }
  }
}

async function ensureRunner({ targetOS, binDir, tempDir }) {
  const defaultSpec = runnerSpecByPlatform[targetOS];
  if (!defaultSpec) {
    throw new Error(`不支持的平台: ${targetOS}`);
  }

  const outputName = targetOS === "windows" ? "onnx_ocr_runner.exe" : "onnx_ocr_runner";
  const outputPath = path.join(binDir, outputName);
  if (await fileExists(outputPath)) {
    console.log(`[prepare-onnx-ocr] runner exists: ${outputName}`);
    return;
  }

  const archiveUrl = process.env.COUNTCAR_OCR_RUNNER_URL || defaultSpec.archiveUrl;
  const archiveInnerPath = process.env.COUNTCAR_OCR_RUNNER_ARCHIVE_PATH || defaultSpec.archiveInnerPath;
  const archivePath = path.join(tempDir, path.basename(new URL(archiveUrl).pathname || "runner.7z"));

  console.log(`[prepare-onnx-ocr] downloading runner archive: ${archiveUrl}`);
  await downloadToFile(archiveUrl, archivePath);

  // 使用系统7z解压指定文件，兼容Linux/Windows构建机。
  await runCommand("7z", ["e", "-y", archivePath, archiveInnerPath, `-o${tempDir}`]);

  const extractedPath = path.join(tempDir, path.basename(archiveInnerPath));
  if (!(await fileExists(extractedPath))) {
    throw new Error(`未找到解压后的runner文件: ${archiveInnerPath}`);
  }

  await fsp.copyFile(extractedPath, outputPath);
  if (targetOS !== "windows") {
    await fsp.chmod(outputPath, 0o755);
  }
  console.log(`[prepare-onnx-ocr] runner prepared: ${outputPath}`);
}

async function main() {
  const args = parseArgs(process.argv);
  const targetOS = (args.os || mapNodePlatformToTarget(process.platform)).toLowerCase();
  const distRoot = path.resolve(PROJECT_ROOT, args.dist || "dist");
  const ocrRoot = path.join(distRoot, "ocr");
  const binDir = path.join(ocrRoot, "bin");
  const modelsDir = path.join(ocrRoot, "models");

  await ensureDir(binDir);
  await ensureDir(modelsDir);

  const tempDir = await fsp.mkdtemp(path.join(os.tmpdir(), "countcar-onnx-"));
  try {
    await ensureRunner({ targetOS, binDir, tempDir });

    const detModelPath = path.join(modelsDir, "license_det.onnx");
    const clsModelPath = path.join(modelsDir, "license_cls.onnx");
    const recModelPath = path.join(modelsDir, "license_rec.onnx");
    const keysPath = path.join(modelsDir, "ppocr_keys_v1.txt");

    await ensureModel(detModelPath, {
      url: process.env.COUNTCAR_OCR_DET_MODEL_URL || defaultDetModel.url,
      sha256: process.env.COUNTCAR_OCR_DET_MODEL_SHA256 || defaultDetModel.sha256,
    });

    await ensureModel(clsModelPath, {
      url: process.env.COUNTCAR_OCR_CLS_MODEL_URL || defaultClsModel.url,
      sha256: process.env.COUNTCAR_OCR_CLS_MODEL_SHA256 || defaultClsModel.sha256,
    });

    await ensureModel(recModelPath, {
      url: process.env.COUNTCAR_OCR_REC_MODEL_URL || defaultRecModel.url,
      sha256: process.env.COUNTCAR_OCR_REC_MODEL_SHA256 || defaultRecModel.sha256,
    });

    await ensureModel(keysPath, {
      url: process.env.COUNTCAR_OCR_KEYS_URL || defaultKeysFile.url,
      sha256: process.env.COUNTCAR_OCR_KEYS_SHA256 || "",
    });

    console.log(`[prepare-onnx-ocr] completed: ${ocrRoot}`);
  } finally {
    await fsp.rm(tempDir, { recursive: true, force: true });
  }
}

main().catch((err) => {
  console.error(`[prepare-onnx-ocr] failed: ${err.message}`);
  process.exit(1);
});
