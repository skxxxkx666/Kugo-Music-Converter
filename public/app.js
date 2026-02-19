import { readSseStream } from "./modules/sse.js";
import { createScanner } from "./modules/scanner.js";

const APP_VERSION = "v0.2.2.1";
const HISTORY_KEY = "kgg-converter-history";
const THEME_KEY = "kgg-converter-theme";
const SUPPORTED_EXTS = [".kgg", ".kgm", ".kgma", ".vpr", ".ncm"];

const STORAGE_KEYS = {
  outputDir: "kgg-converter-output-dir",
  dbPath: "kgg-converter-db-path",
  outputFormat: "kgg-converter-output-format",
  mp3Quality: "kgg-converter-mp3-quality",
  concurrency: "kgg-converter-concurrency"
};

const LOG_LEVEL_LABELS = {
  error: "ERROR",
  success: "OK",
  warn: "WARN",
  info: "INFO"
};
const MAX_LOG_ENTRIES = 500;

const fileInput = document.getElementById("kggFiles");
const pickFilesBtn = document.getElementById("pickFilesBtn");
const dropZone = document.getElementById("dropZone");
const filePreviewList = document.getElementById("filePreviewList");
const fileSummary = document.getElementById("fileSummary");

const outputDirInput = document.getElementById("outputDir");
const dbPathInput = document.getElementById("dbPath");
const outputFormatSelect = document.getElementById("outputFormat");
const mp3QualitySelect = document.getElementById("mp3Quality");
const mp3QualityWrap = document.getElementById("mp3QualityWrap");
const concurrencySelect = document.getElementById("concurrency");

const pickDirBtn = document.getElementById("pickDirBtn");
const openDirBtn = document.getElementById("openDirBtn");
const pickDbBtn = document.getElementById("pickDbBtn");
const redetectDbBtn = document.getElementById("redetectDbBtn");
const convertBtn = document.getElementById("convertBtn");
const cancelBtn = document.getElementById("cancelBtn");
const openOutputBtn = document.getElementById("openOutputBtn");
const openResultDirBtn = document.getElementById("openResultDirBtn");

const globalAlert = document.getElementById("globalAlert");
const dbStatus = document.getElementById("dbStatus");
const runtimeStatus = document.getElementById("runtimeStatus");
const logBox = document.getElementById("logBox");
const exportLogBtn = document.getElementById("exportLogBtn");

const progressSection = document.getElementById("progressSection");
const totalProgressBar = document.getElementById("totalProgressBar");
const totalProgressText = document.getElementById("totalProgressText");
const progressStatus = document.getElementById("progressStatus");
const progressETA = document.getElementById("progressETA");
const fileProgressList = document.getElementById("fileProgressList");

const resultDashboard = document.getElementById("resultDashboard");
const statSuccess = document.getElementById("statSuccess");
const statFailed = document.getElementById("statFailed");
const statDuration = document.getElementById("statDuration");
const failedSection = document.getElementById("failedSection");
const failedList = document.getElementById("failedList");
const exportFailedBtn = document.getElementById("exportFailedBtn");

const historyPanel = document.getElementById("historyPanel");
const clearHistoryBtn = document.getElementById("clearHistoryBtn");

const pickFoldersBtn = document.getElementById("pickFoldersBtn");
const scanRecursive = document.getElementById("scanRecursive");
const selectedFolders = document.getElementById("selectedFolders");
const extFilter = document.getElementById("extFilter");
const customExtWrap = document.getElementById("customExtWrap");
const customExtFilter = document.getElementById("customExtFilter");
const scanBtn = document.getElementById("scanBtn");
const scanResult = document.getElementById("scanResult");
const scanTotal = document.getElementById("scanTotal");
const scanSize = document.getElementById("scanSize");
const fileNameList = document.getElementById("fileNameList");
const copyNamesBtn = document.getElementById("copyNamesBtn");
const copyPathsBtn = document.getElementById("copyPathsBtn");
const exportCsvBtn = document.getElementById("exportCsvBtn");
const selectAllForConvert = document.getElementById("selectAllForConvert");

const themeToggleBtn = document.getElementById("themeToggleBtn");
const versionBadge = document.getElementById("versionBadge");
const footerVersion = document.getElementById("footerVersion");

const state = {
  isBusy: false,
  selectedFiles: [],
  pathQueue: [],
  selectedFolderPaths: [],
  scanFiles: [],
  missingTools: [],
  autoDbFound: false,
  manualDbValid: false,
  maxFileCount: 500,
  maxFileSizeMB: 80,
  supportedFormats: SUPPORTED_EXTS,
  startedAt: 0,
  hasFileError: false,
  fileRowMap: new Map(),
  lastSummary: null,
  history: [],
  abortController: null,
  progressDone: 0,
  progressTotal: 0,
  failedResults: []
};

let validateDbTimer = null;
let useSystemThemeSync = true;

function appendLog(level, message) {
  const now = new Date().toLocaleTimeString("zh-CN", { hour12: false });
  const line = document.createElement("div");
  line.className = `log-line log-${level}`;
  const label = LOG_LEVEL_LABELS[level] || "INFO";
  line.textContent = `[${now}] [${label}] ${message}`;
  logBox.appendChild(line);
  while (logBox.children.length > MAX_LOG_ENTRIES) {
    logBox.removeChild(logBox.firstChild);
  }
  logBox.scrollTop = logBox.scrollHeight;
}

function severityToLogLevel(severity) {
  const normalized = String(severity || "").toLowerCase();
  if (normalized === "fatal" || normalized === "error") return "error";
  if (normalized === "warning" || normalized === "warn") return "warn";
  return "info";
}

function appendPayloadError(prefix, payload) {
  const userMessage = payload?.userMessage || payload?.error || "发生未知错误";
  appendLog(severityToLogLevel(payload?.severity), `${prefix}${userMessage}`);
  if (payload?.suggestion) appendLog("warn", `建议：${payload.suggestion}`);
}

function setHintStatus(element, statusType, text) {
  element.className = "hint";
  if (statusType) element.classList.add(`status-${statusType}`);
  element.textContent = text;
}

function getExt(name) {
  const idx = String(name || "").lastIndexOf(".");
  return idx === -1 ? "" : String(name).slice(idx).toLowerCase();
}

function formatBytes(bytes) {
  if (!bytes) return "0 B";
  const kb = 1024;
  const mb = kb * 1024;
  const gb = mb * 1024;
  if (bytes >= gb) return `${(bytes / gb).toFixed(1)} GB`;
  if (bytes >= mb) return `${(bytes / mb).toFixed(1)} MB`;
  if (bytes >= kb) return `${(bytes / kb).toFixed(1)} KB`;
  return `${bytes} B`;
}

function formatDuration(ms) {
  const sec = Math.max(0, Math.round(ms / 1000));
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return `${m}分${String(s).padStart(2, "0")}秒`;
}

function formatEtaBySpeed(done, total) {
  if (!state.startedAt || done <= 0 || total <= 0 || done >= total) return "";
  const elapsedSec = (Date.now() - state.startedAt) / 1000;
  if (elapsedSec <= 0) return "";
  const filesPerSec = done / elapsedSec;
  if (filesPerSec <= 0) return "";
  const remainSec = Math.max(0, Math.round((total - done) / filesPerSec));
  const m = Math.floor(remainSec / 60);
  const s = remainSec % 60;
  return `预计剩余：${m}:${String(s).padStart(2, "0")}`;
}

function updateProgressETA() {
  progressETA.textContent = formatEtaBySpeed(state.progressDone, state.progressTotal);
}

function updateProgressBar(percent, hasError = false) {
  const safe = Math.max(0, Math.min(100, Number(percent) || 0));
  totalProgressBar.style.width = `${safe}%`;
  totalProgressText.textContent = `${safe}%`;
  totalProgressBar.classList.toggle("error", Boolean(hasError));
}

function extBadgeClass(ext) {
  const normalized = String(ext || "")
    .replace(".", "")
    .toLowerCase()
    .replace(/[^a-z0-9_-]/g, "");
  return `file-ext-badge ext-${normalized || "unknown"}`;
}

function escapeHtml(str) {
  const div = document.createElement("div");
  div.textContent = String(str ?? "");
  return div.innerHTML;
}

async function fetchJson(url, options = {}) {
  const response = await fetch(url, options);
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const message = data.userMessage || data.error || data.message || "请求失败，请稍后重试";
    const error = new Error(message);
    error.payload = data;
    throw error;
  }
  return data;
}

function getPendingItems() {
  const uploadItems = state.selectedFiles.map((file, index) => ({
    source: "upload",
    index,
    name: file.name,
    size: file.size,
    ext: getExt(file.name)
  }));
  const pathItems = state.pathQueue.map((item, index) => ({
    source: "path",
    index,
    name: item.name,
    size: item.size,
    ext: item.ext,
    fullPath: item.fullPath
  }));
  return [...uploadItems, ...pathItems];
}

function pendingCount() {
  return state.selectedFiles.length + state.pathQueue.length;
}

function requiresDb() {
  return getPendingItems().some((item) => item.ext === ".kgg");
}

function isDbReady() {
  return state.autoDbFound || state.manualDbValid;
}

function renderFilePreview() {
  const items = getPendingItems();
  filePreviewList.innerHTML = "";
  if (items.length === 0) return;

  items.forEach((item) => {
    const extText = (item.ext || ".").replace(".", "").toUpperCase();
    const titleText = item.source === "path" ? item.fullPath : item.name;
    const prefix = item.source === "path" ? "[路径]" : "[上传]";
    const displayName = `${prefix} ${item.name}`;
    const row = document.createElement("div");
    row.className = "file-preview-item";
    row.innerHTML = `
      <span class="${extBadgeClass(item.ext)}">${escapeHtml(extText)}</span>
      <span class="file-preview-name" title="${escapeHtml(titleText)}">
        ${escapeHtml(displayName)}
      </span>
      <span class="file-preview-size">${formatBytes(item.size)}</span>
      <button class="btn-remove" data-source="${item.source}" data-index="${item.index}" title="移除">x</button>
    `;
    filePreviewList.appendChild(row);
  });
}

function updateFileSummary() {
  const items = getPendingItems();
  if (items.length === 0) {
    setHintStatus(fileSummary, "", "未选择文件");
    return;
  }

  const totalBytes = items.reduce((sum, item) => sum + (item.size || 0), 0);
  if (items.length > state.maxFileCount) {
    setHintStatus(fileSummary, "error", `已选择 ${items.length} 个文件（超过上限 ${state.maxFileCount}）`);
    return;
  }

  setHintStatus(
    fileSummary,
    "success",
    `已选择 ${items.length} 个文件，总大小 ${formatBytes(totalBytes)}（上传 ${state.selectedFiles.length}，路径队列 ${state.pathQueue.length}）`
  );
}

function renderGlobalAlert() {
  const issues = [];
  if (state.missingTools.length > 0) issues.push(`缺少运行时文件：${state.missingTools.join("、")}`);
  if (requiresDb() && !isDbReady()) issues.push("当前队列包含 KGG 文件，但未检测到可用 KGMusicV3.db。");

  if (issues.length === 0) {
    globalAlert.classList.add("hidden");
    globalAlert.textContent = "";
    return;
  }

  globalAlert.classList.remove("hidden");
  globalAlert.textContent = issues.join("\n");
}

function updateConvertButtonState() {
  const ready =
    !state.isBusy &&
    pendingCount() > 0 &&
    state.missingTools.length === 0 &&
    outputDirInput.value.trim() &&
    (!requiresDb() || isDbReady());

  convertBtn.disabled = !ready;
  convertBtn.textContent = state.isBusy ? "转换中..." : "开始转换";
  cancelBtn.disabled = !state.isBusy;
}

function setBusy(isBusy) {
  state.isBusy = isBusy;
  fileInput.disabled = isBusy;
  pickFilesBtn.disabled = isBusy;
  outputDirInput.disabled = isBusy;
  dbPathInput.disabled = isBusy;
  outputFormatSelect.disabled = isBusy;
  mp3QualitySelect.disabled = isBusy;
  concurrencySelect.disabled = isBusy;
  pickDirBtn.disabled = isBusy;
  pickDbBtn.disabled = isBusy;
  redetectDbBtn.disabled = isBusy;
  pickFoldersBtn.disabled = isBusy;
  scanBtn.disabled = isBusy || state.selectedFolderPaths.length === 0;
  updateConvertButtonState();
}

function savePreferences() {
  localStorage.setItem(STORAGE_KEYS.outputDir, outputDirInput.value.trim());
  localStorage.setItem(STORAGE_KEYS.dbPath, dbPathInput.value.trim());
  localStorage.setItem(STORAGE_KEYS.outputFormat, outputFormatSelect.value);
  localStorage.setItem(STORAGE_KEYS.mp3Quality, mp3QualitySelect.value);
  localStorage.setItem(STORAGE_KEYS.concurrency, concurrencySelect.value);
}

function loadPreferences() {
  const outputDir = localStorage.getItem(STORAGE_KEYS.outputDir);
  const dbPath = localStorage.getItem(STORAGE_KEYS.dbPath);
  const outputFormat = localStorage.getItem(STORAGE_KEYS.outputFormat);
  const mp3Quality = localStorage.getItem(STORAGE_KEYS.mp3Quality);
  const concurrency = localStorage.getItem(STORAGE_KEYS.concurrency);

  if (outputDir) outputDirInput.value = outputDir;
  if (dbPath) dbPathInput.value = dbPath;
  if (outputFormat) outputFormatSelect.value = outputFormat;
  if (mp3Quality) mp3QualitySelect.value = mp3Quality;
  if (concurrency) concurrencySelect.value = concurrency;
}

function loadHistory() {
  try {
    state.history = JSON.parse(localStorage.getItem(HISTORY_KEY) || "[]");
  } catch {
    state.history = [];
  }
}

function saveHistory(summary) {
  const history = Array.isArray(state.history) ? [...state.history] : [];
  history.unshift({
    timestamp: new Date().toISOString(),
    total: summary.total || 0,
    success: summary.success || 0,
    failed: summary.failed || 0,
    durationMs: summary.durationMs || 0,
    outputDir: summary.outputDir || outputDirInput.value.trim(),
    outputFormat: summary.outputFormat || outputFormatSelect.value
  });
  if (history.length > 50) history.length = 50;
  state.history = history;
  localStorage.setItem(HISTORY_KEY, JSON.stringify(history));
}

function renderHistory() {
  historyPanel.innerHTML = "";
  if (!state.history.length) {
    historyPanel.innerHTML = '<div class="history-empty">暂无历史记录</div>';
    return;
  }

  state.history.slice(0, 10).forEach((item) => {
    const row = document.createElement("div");
    row.className = "history-item";
    const timeText = new Date(item.timestamp).toLocaleString("zh-CN", { hour12: false });
    const outputFormat = String(item.outputFormat || "").toUpperCase();
    row.innerHTML = `
      <div class="history-main">${escapeHtml(timeText)}</div>
      <div class="history-sub">文件 ${escapeHtml(item.total)} | 成功 ${escapeHtml(item.success)} | 失败 ${escapeHtml(item.failed)} | ${escapeHtml(formatDuration(item.durationMs))} | ${escapeHtml(outputFormat)}</div>
    `;
    historyPanel.appendChild(row);
  });
}

function updateVersionBadge() {
  if (versionBadge) versionBadge.textContent = `版本 ${APP_VERSION}`;
  if (footerVersion) footerVersion.textContent = APP_VERSION;
}

function applyTheme(theme, persist = true) {
  const normalized = theme === "dark" ? "dark" : "light";
  document.documentElement.setAttribute("data-theme", normalized);
  themeToggleBtn.textContent = normalized === "dark" ? "切换浅色" : "切换深色";
  if (persist) localStorage.setItem(THEME_KEY, normalized);
}

function initTheme() {
  const saved = localStorage.getItem(THEME_KEY);
  useSystemThemeSync = !saved;

  if (saved === "light" || saved === "dark") {
    applyTheme(saved, false);
  } else {
    const prefersDark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
    applyTheme(prefersDark ? "dark" : "light", false);
  }

  if (window.matchMedia) {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    media.addEventListener("change", (e) => {
      if (useSystemThemeSync) applyTheme(e.matches ? "dark" : "light", false);
    });
  }
}

function updateMp3QualityVisibility() {
  mp3QualityWrap.classList.toggle("hidden", outputFormatSelect.value !== "mp3");
}

function queueChanged() {
  renderFilePreview();
  updateFileSummary();
  renderGlobalAlert();
  updateConvertButtonState();
}

function applyConfig(config) {
  state.missingTools = Array.isArray(config.missingTools) ? config.missingTools : [];

  if (config.limits) {
    state.maxFileCount = Number(config.limits.maxFileCount) || state.maxFileCount;
    state.maxFileSizeMB = Number(config.limits.maxFileSizeMB) || state.maxFileSizeMB;
  }

  if (Array.isArray(config.supportedFormats) && config.supportedFormats.length > 0) {
    state.supportedFormats = config.supportedFormats.map((item) => String(item).toLowerCase());
  }

  if (!outputDirInput.value.trim()) outputDirInput.value = config.defaultOutputDir || "";

  if (config.db?.found) {
    state.autoDbFound = true;
    if (!dbPathInput.value.trim()) dbPathInput.value = config.db.path;

    const sourceMap = {
      project: "项目目录",
      appdata: "AppData",
      localappdata: "LocalAppData",
      manual: "手动配置",
      request: "请求参数"
    };
    setHintStatus(dbStatus, "success", `数据库已就绪（来源：${sourceMap[config.db.source] || config.db.source}）`);
  } else {
    state.autoDbFound = false;
    if (!state.manualDbValid) setHintStatus(dbStatus, "warn", "未检测到 KGMusicV3.db（仅 KGG 文件需要）。");
  }

  if (state.missingTools.length > 0) {
    setHintStatus(runtimeStatus, "error", `运行时缺失：${state.missingTools.join("、")}`);
  } else {
    setHintStatus(
      runtimeStatus,
      "success",
      `运行时检查通过。限制：最多 ${state.maxFileCount} 个文件，单文件 ${state.maxFileSizeMB}MB。`
    );
  }

  renderGlobalAlert();
  updateConvertButtonState();
}

async function loadConfig() {
  try {
    const config = await fetchJson("/api/config");
    applyConfig(config);
  } catch (err) {
    appendLog("error", `加载配置失败：${err.message}`);
  }
}

async function validateDbPath(value) {
  const input = String(value || "").trim();
  if (!input) {
    state.manualDbValid = false;
    if (state.autoDbFound) setHintStatus(dbStatus, "success", "数据库已自动检测到。");
    else setHintStatus(dbStatus, "warn", "请先选择 KGMusicV3.db（仅 KGG 需要）。");
    renderGlobalAlert();
    updateConvertButtonState();
    return;
  }

  try {
    const data = await fetchJson("/api/validate-db-path", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ dbPath: input })
    });

    if (data.valid) {
      state.manualDbValid = true;
      setHintStatus(dbStatus, "success", `数据库路径有效：${data.path}`);
    } else {
      state.manualDbValid = false;
      setHintStatus(dbStatus, "error", "数据库路径无效，请确认文件存在且文件名为 KGMusicV3.db。");
    }
  } catch (err) {
    state.manualDbValid = false;
    setHintStatus(dbStatus, "error", `数据库路径校验失败：${err.message}`);
  }

  renderGlobalAlert();
  updateConvertButtonState();
}

function scheduleDbValidation() {
  clearTimeout(validateDbTimer);
  validateDbTimer = setTimeout(() => {
    validateDbPath(dbPathInput.value);
  }, 250);
}

function mergeFiles(newFiles) {
  const incoming = Array.from(newFiles || []);
  if (incoming.length === 0) return;

  const signatures = new Set(state.selectedFiles.map((file) => `${file.name}|${file.size}|${file.lastModified}`));

  for (const file of incoming) {
    const ext = getExt(file.name);
    if (!state.supportedFormats.includes(ext)) {
      appendLog("warn", `已跳过不支持的文件：${file.name}`);
      continue;
    }
    if (file.size > state.maxFileSizeMB * 1024 * 1024) {
      appendLog("warn", `文件过大已跳过：${file.name}`);
      continue;
    }

    const sign = `${file.name}|${file.size}|${file.lastModified}`;
    if (signatures.has(sign)) continue;

    state.selectedFiles.push(file);
    signatures.add(sign);
  }

  if (pendingCount() > state.maxFileCount) {
    state.selectedFiles = state.selectedFiles.slice(0, Math.max(0, state.maxFileCount - state.pathQueue.length));
    appendLog("warn", `已超过文件上限，自动截断为最多 ${state.maxFileCount} 个。`);
  }

  queueChanged();
}

async function pickOutputDir() {
  try {
    appendLog("info", "正在打开输出目录选择器...");
    const data = await fetchJson("/api/pick-directory", { method: "POST" });
    if (data.path) {
      outputDirInput.value = data.path;
      savePreferences();
      appendLog("success", `输出目录：${data.path}`);
      updateConvertButtonState();
    }
  } catch (err) {
    appendLog("error", `选择输出目录失败：${err.message}`);
  }
}

async function openOutputFolder() {
  const dir = outputDirInput.value.trim();
  if (!dir) {
    appendLog("warn", "请先设置输出目录。");
    return;
  }

  try {
    await fetchJson("/api/open-folder", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path: dir })
    });
  } catch (err) {
    appendLog("error", `打开文件夹失败：${err.message}`);
  }
}

async function pickDbFile() {
  try {
    appendLog("info", "正在打开 KGMusicV3.db 选择窗口...");
    const data = await fetchJson("/api/pick-db-file", { method: "POST" });
    if (data.path) {
      dbPathInput.value = data.path;
      state.manualDbValid = true;
      setHintStatus(dbStatus, "success", `数据库路径：${data.path}`);
      savePreferences();
      appendLog("success", "KGMusicV3.db 已选择。");
    }
    renderGlobalAlert();
    updateConvertButtonState();
  } catch (err) {
    appendLog("error", `选择数据库失败：${err.message}`);
  }
}

async function redetectDb() {
  try {
    appendLog("info", "正在重新检测 KGMusicV3.db...");
    const data = await fetchJson("/api/redetect-db", { method: "POST" });

    if (data.db?.found) {
      state.autoDbFound = true;
      if (!dbPathInput.value.trim()) dbPathInput.value = data.db.path;
      setHintStatus(dbStatus, "success", `自动检测到数据库：${data.db.path}`);
      appendLog("success", "已重新检测到 KGMusicV3.db。");
    } else {
      state.autoDbFound = false;
      if (state.manualDbValid) setHintStatus(dbStatus, "success", "当前使用手动配置数据库。");
      else setHintStatus(dbStatus, "warn", "仍未检测到 KGMusicV3.db。");
      appendLog("warn", "自动检测未找到 KGMusicV3.db。");
    }

    const config = await fetchJson("/api/config");
    applyConfig(config);
  } catch (err) {
    appendLog("error", `重新检测失败：${err.message}`);
  }
}

function renderDashboard(summary) {
  resultDashboard.classList.remove("hidden");
  statSuccess.textContent = String(summary.success || 0);
  statFailed.textContent = String(summary.failed || 0);
  statDuration.textContent = formatDuration(summary.durationMs || 0);
}

function renderFailedDetails(results) {
  const failed = (results || []).filter((item) => item?.status === "error");
  state.failedResults = failed;
  failedList.innerHTML = "";

  if (failed.length === 0) {
    failedSection.classList.add("hidden");
    return;
  }

  failed.forEach((item, idx) => {
    const row = document.createElement("div");
    row.className = "failed-item";
    const err = item.error || {};
    const fileText = item.file || "未知文件";
    const errCode = err.code || "ERR_UNKNOWN";
    const reason = err.userMessage || err.detail || "转换失败";
    const suggestion = err.suggestion || "请查看日志后重试";
    const inputPath = item.input || "（上传文件，未提供原始绝对路径）";
    row.innerHTML = `
      <div class="failed-main">${escapeHtml(`${idx + 1}. ${fileText}`)}</div>
      <div class="failed-meta">错误码：${escapeHtml(errCode)}</div>
      <div class="failed-meta">原因：${escapeHtml(reason)}</div>
      <div class="failed-meta">建议：${escapeHtml(suggestion)}</div>
      <div class="failed-path">源路径：${escapeHtml(inputPath)}</div>
    `;
    failedList.appendChild(row);
  });

  failedSection.classList.remove("hidden");
}

function resetProgressUI(total) {
  progressSection.classList.remove("hidden");
  fileProgressList.innerHTML = "";
  state.fileRowMap.clear();
  state.startedAt = Date.now();
  state.hasFileError = false;
  state.lastSummary = null;
  state.progressDone = 0;
  state.progressTotal = total;
  state.failedResults = [];

  failedSection.classList.add("hidden");
  failedList.innerHTML = "";

  updateProgressBar(0, false);
  progressStatus.textContent = "准备开始转换...";
  progressETA.textContent = "";
  resultDashboard.classList.add("hidden");
}

function getFileRowKey(data) {
  return `${data.current}-${data.file}`;
}

function getOrCreateFileRow(data) {
  const key = getFileRowKey(data);
  if (state.fileRowMap.has(key)) return state.fileRowMap.get(key);

  const row = document.createElement("div");
  row.className = "file-list-item pending";

  const icon = document.createElement("span");
  icon.className = "status-icon";
  icon.textContent = "等待";

  const text = document.createElement("span");
  text.className = "file-text";
  text.textContent = `[${data.current}/${data.total}] ${data.file}`;

  row.appendChild(icon);
  row.appendChild(text);
  fileProgressList.appendChild(row);

  const payload = { row, icon, text };
  state.fileRowMap.set(key, payload);
  return payload;
}

function updateFileRow(data, statusClass, statusText) {
  const item = getOrCreateFileRow(data);
  item.row.className = `file-list-item ${statusClass}`;
  if (statusClass === "active") item.icon.textContent = "进行";
  if (statusClass === "success") item.icon.textContent = "完成";
  if (statusClass === "error") item.icon.textContent = "失败";
  if (statusClass === "pending") item.icon.textContent = "等待";
  item.text.textContent = `[${data.current}/${data.total}] ${data.file} ${statusText}`;
}

function phaseText(phase) {
  if (phase === "prepare") return "准备中";
  if (phase === "decrypt") return "解密中";
  if (phase === "transcode") return "转码中";
  return "处理中";
}

function handleProgressEvent(eventName, data) {
  if (eventName === "progress") {
    progressStatus.textContent = `${phaseText(data.phase)}：${data.file} (${data.current}/${data.total})`;
    updateProgressBar(data.percent, state.hasFileError);
    updateFileRow(data, "active", `- ${phaseText(data.phase)}...`);
    return;
  }

  if (eventName === "file-done") {
    state.progressDone += 1;
    updateProgressETA();
    updateProgressBar(data.percent, state.hasFileError || data.status === "error");

    if (data.status === "ok") {
      updateFileRow(data, "success", "- 转换成功");
      appendLog("success", `转换成功：${data.file}`);
    } else {
      state.hasFileError = true;
      const userMsg = data.error?.userMessage || "转换失败";
      updateFileRow(data, "error", `- ${userMsg}`);
      appendPayloadError(`转换失败：${data.file} | `, data.error);
    }
    return;
  }

  if (eventName === "error") {
    appendPayloadError("流式转换失败：", data.error);
    return;
  }

  if (eventName === "complete") {
    state.lastSummary = data;
    const doneText = `已完成：成功 ${data.success || 0}，失败 ${data.failed || 0}，耗时 ${formatDuration(data.durationMs || 0)}`;
    progressStatus.textContent = doneText;
    progressETA.textContent = "";
    appendLog("info", doneText);
    if (data.cancelled) appendLog("warn", "任务已取消。");

    renderDashboard(data);
    renderFailedDetails(data.results || []);
    saveHistory(data);
    renderHistory();
    notifyConvertComplete(data);
  }
}

async function startConvertStream(formData, signal) {
  const response = await fetch("/api/convert-stream", { method: "POST", body: formData, signal });
  const contentType = response.headers.get("content-type") || "";

  if (!response.ok || !contentType.includes("text/event-stream")) {
    const data = await response.json().catch(() => ({}));
    const message = data.userMessage || data.error || "转换请求失败";
    const error = new Error(message);
    error.payload = data;
    throw error;
  }

  await readSseStream(response, handleProgressEvent);
}

function playCompleteTone() {
  const AudioCtx = window.AudioContext || window.webkitAudioContext;
  if (!AudioCtx) return;

  try {
    const ctx = new AudioCtx();
    const oscillator = ctx.createOscillator();
    const gain = ctx.createGain();

    oscillator.type = "triangle";
    oscillator.frequency.value = 880;
    gain.gain.value = 0.03;

    oscillator.connect(gain);
    gain.connect(ctx.destination);

    const now = ctx.currentTime;
    oscillator.start(now);
    oscillator.stop(now + 0.18);
    oscillator.onended = () => {
      ctx.close().catch(() => {});
    };
  } catch {
    // ignore audio failures
  }
}

function notifyConvertComplete(summary) {
  const title = "转换任务已完成";
  const body = `成功 ${summary.success || 0}，失败 ${summary.failed || 0}，耗时 ${formatDuration(summary.durationMs || 0)}`;

  playCompleteTone();

  if (!("Notification" in window)) return;
  if (Notification.permission === "granted") {
    // eslint-disable-next-line no-new
    new Notification(title, { body });
    return;
  }
  if (Notification.permission === "default") {
    Notification.requestPermission().then((permission) => {
      if (permission === "granted") {
        // eslint-disable-next-line no-new
        new Notification(title, { body });
      }
    });
  }
}

async function startConvert() {
  const items = getPendingItems();
  const outputDir = outputDirInput.value.trim();
  const dbPath = dbPathInput.value.trim();

  if (items.length === 0) {
    appendLog("warn", "请先选择文件或加入路径队列。");
    return;
  }
  if (items.length > state.maxFileCount) {
    appendLog("warn", `文件数量超过限制（最多 ${state.maxFileCount} 个）。`);
    return;
  }
  if (!outputDir) {
    appendLog("warn", "请先选择输出目录。");
    return;
  }
  if (requiresDb() && !dbPath && !isDbReady()) {
    appendLog("warn", "当前队列包含 KGG，请先配置 KGMusicV3.db。");
    return;
  }

  const formData = new FormData();
  formData.append("outputDir", outputDir);
  formData.append("outputFormat", outputFormatSelect.value);
  formData.append("mp3Quality", mp3QualitySelect.value);
  formData.append("concurrency", concurrencySelect.value);

  if (dbPath) formData.append("dbPath", dbPath);
  for (const file of state.selectedFiles) formData.append("kggFiles", file, file.name);
  if (state.pathQueue.length > 0) {
    formData.append("inputPaths", JSON.stringify(state.pathQueue.map((item) => item.fullPath)));
  }

  resetProgressUI(items.length);
  appendLog("info", `开始转换，共 ${items.length} 个文件...`);

  try {
    state.abortController = new AbortController();
    setBusy(true);
    await startConvertStream(formData, state.abortController.signal);
    if (state.lastSummary) appendLog("info", `输出目录：${state.lastSummary.outputDir || outputDir}`);
  } catch (err) {
    if (err.name === "AbortError") appendLog("warn", "用户已取消转换。");
    else if (err?.payload) appendPayloadError("转换失败：", err.payload);
    else appendLog("error", `转换失败：${err.message}`);
  } finally {
    state.abortController = null;
    setBusy(false);
  }
}

function cancelConvert() {
  if (state.abortController) {
    state.abortController.abort();
    appendLog("warn", "用户已取消转换。");
  }
}

function csvEscape(value) {
  return `"${String(value ?? "").replace(/"/g, '""')}"`;
}

function exportFailedList() {
  if (!state.failedResults.length) {
    appendLog("warn", "暂无失败文件可导出。");
    return;
  }

  const rows = [
    "文件名,源路径,错误码,错误信息,建议",
    ...state.failedResults.map((item) => {
      const err = item.error || {};
      return [
        csvEscape(item.file || ""),
        csvEscape(item.input || ""),
        csvEscape(err.code || "ERR_UNKNOWN"),
        csvEscape(err.userMessage || err.detail || "转换失败"),
        csvEscape(err.suggestion || "")
      ].join(",");
    })
  ];

  const blob = new Blob(["\uFEFF" + rows.join("\n")], { type: "text/csv;charset=utf-8" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = `失败文件列表_${new Date().toISOString().replace(/[:.]/g, "-")}.csv`;
  a.click();
  URL.revokeObjectURL(a.href);
  appendLog("success", "失败文件列表已导出。");
}

function exportLogs() {
  const lines = Array.from(logBox.querySelectorAll(".log-line")).map((line) => line.textContent);
  const blob = new Blob([lines.join("\n")], { type: "text/plain;charset=utf-8" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = `转换日志_${new Date().toISOString().replace(/[:.]/g, "-")}.txt`;
  a.click();
  URL.revokeObjectURL(a.href);
}

async function copyTextToClipboard(text, okMessage) {
  try {
    await navigator.clipboard.writeText(text);
    appendLog("success", okMessage);
  } catch {
    appendLog("error", "复制失败，请检查剪贴板权限。");
  }
}

const scanner = createScanner({
  state,
  elements: {
    selectedFolders,
    scanBtn,
    scanRecursive,
    extFilter,
    customExtWrap,
    customExtFilter,
    scanResult,
    scanTotal,
    scanSize,
    fileNameList
  },
  fetchJson,
  appendLog,
  appendPayloadError,
  formatBytes,
  extBadgeClass,
  copyTextToClipboard,
  onQueueChanged: queueChanged,
  pendingCount
});

function bindEvents() {
  pickFilesBtn.addEventListener("click", () => fileInput.click());
  fileInput.addEventListener("change", () => {
    mergeFiles(fileInput.files);
    fileInput.value = "";
  });

  dropZone.addEventListener("dragover", (e) => {
    e.preventDefault();
    dropZone.classList.add("drag-over");
  });
  dropZone.addEventListener("dragleave", () => {
    dropZone.classList.remove("drag-over");
  });
  dropZone.addEventListener("drop", (e) => {
    e.preventDefault();
    dropZone.classList.remove("drag-over");
    mergeFiles(e.dataTransfer.files);
  });

  filePreviewList.addEventListener("click", (e) => {
    const btn = e.target.closest(".btn-remove");
    if (!btn) return;
    const source = btn.getAttribute("data-source");
    const index = Number.parseInt(btn.getAttribute("data-index"), 10);
    if (!Number.isFinite(index)) return;
    if (source === "upload") state.selectedFiles.splice(index, 1);
    if (source === "path") state.pathQueue.splice(index, 1);
    queueChanged();
  });

  pickDirBtn.addEventListener("click", pickOutputDir);
  openDirBtn.addEventListener("click", openOutputFolder);
  openOutputBtn.addEventListener("click", openOutputFolder);
  openResultDirBtn.addEventListener("click", openOutputFolder);
  pickDbBtn.addEventListener("click", pickDbFile);
  redetectDbBtn.addEventListener("click", redetectDb);
  convertBtn.addEventListener("click", startConvert);
  cancelBtn.addEventListener("click", cancelConvert);
  exportLogBtn.addEventListener("click", exportLogs);
  exportFailedBtn.addEventListener("click", exportFailedList);

  outputDirInput.addEventListener("input", () => {
    savePreferences();
    updateConvertButtonState();
  });
  dbPathInput.addEventListener("input", () => {
    savePreferences();
    scheduleDbValidation();
  });
  outputFormatSelect.addEventListener("change", () => {
    savePreferences();
    updateMp3QualityVisibility();
  });
  mp3QualitySelect.addEventListener("change", savePreferences);
  concurrencySelect.addEventListener("change", savePreferences);

  clearHistoryBtn.addEventListener("click", () => {
    state.history = [];
    localStorage.removeItem(HISTORY_KEY);
    renderHistory();
    appendLog("info", "历史记录已清空。");
  });

  themeToggleBtn.addEventListener("click", () => {
    useSystemThemeSync = false;
    const current = document.documentElement.getAttribute("data-theme") || "light";
    applyTheme(current === "dark" ? "light" : "dark");
  });

  pickFoldersBtn.addEventListener("click", scanner.pickFolderForScan);
  selectedFolders.addEventListener("click", (e) => {
    const btn = e.target.closest(".tag-remove");
    if (!btn) return;
    const index = Number.parseInt(btn.getAttribute("data-index"), 10);
    scanner.removeFolder(index);
  });

  extFilter.addEventListener("change", scanner.handleExtFilterChange);
  scanBtn.addEventListener("click", scanner.startScanFolders);
  copyNamesBtn.addEventListener("click", scanner.copyNames);
  copyPathsBtn.addEventListener("click", scanner.copyPaths);
  exportCsvBtn.addEventListener("click", scanner.exportCsv);
  selectAllForConvert.addEventListener("click", scanner.addScanFilesToQueue);
}

(async function init() {
  updateVersionBadge();
  initTheme();
  loadPreferences();
  loadHistory();
  renderHistory();
  bindEvents();
  scanner.renderFolderTags();
  updateMp3QualityVisibility();
  renderFilePreview();
  updateFileSummary();
  setBusy(false);

  await loadConfig();
  if (dbPathInput.value.trim()) scheduleDbValidation();

  appendLog("info", "页面已就绪。");
})();
