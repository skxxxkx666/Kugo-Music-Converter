import { readSseStream } from "./modules/sse.js";
import { createScanner } from "./modules/scanner.js";
import { fadeIn, prefersReducedMotion, slideDown, stagger } from "./modules/animations.js";
import { parseVersion, isPreviewVersion, shouldNotifyUpdate } from "./modules/version.js";
import { createHistoryController } from "./modules/history_controller.js";
import { confirmDestructive, createToastManager, withButtonLoading } from "./modules/feedback.js";
import { createIconToolkit } from "./modules/icons.js";
import { createUpdateController } from "./modules/update.js";
import { createUploaderController } from "./modules/uploader.js";
import { createConfigController } from "./modules/config.js";
import { createConverterController } from "./modules/converter.js";
import { createA11yAnnouncer } from "./modules/a11y.js";
import { createDragDropController } from "./modules/dragdrop.js";
import { escapeHtml, formatBytes, formatDuration } from "./modules/utils.js";

function resolveAppVersion() {
  const htmlVersion = document.documentElement?.getAttribute("data-app-version");
  const runtimeVersion = typeof window !== "undefined" ? window.__APP_VERSION__ : "";
  return String(runtimeVersion || htmlVersion || "dev").trim() || "dev";
}

const APP_VERSION = resolveAppVersion();
const HISTORY_KEY = "kgg-converter-history";
const THEME_KEY = "kgg-converter-theme";
const CONFIG_COLLAPSED_KEY = "kgg-converter-config-collapsed";
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

const EXT_ICON_MAP = {
  ".kgg": "lock",
  ".kgm": "music",
  ".kgma": "music",
  ".vpr": "file-audio",
  ".ncm": "disc",
  ".mp3": "file-audio",
  ".flac": "file-audio",
  ".wav": "file-audio",
  ".ogg": "file-audio"
};

const LUCIDE_FALLBACK_CDNS = [];

const fileInput = document.getElementById("kggFiles");
const pickFilesBtn = document.getElementById("pickFilesBtn");
const dropZone = document.getElementById("dropZone");
const filePreviewList = document.getElementById("filePreviewList");
const fileSummary = document.getElementById("fileSummary");
const clearQueueBtn = document.getElementById("clearQueueBtn");

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
const updateBannerHost = document.getElementById("updateBannerHost");
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
const successSection = document.getElementById("successSection");
const successList = document.getElementById("successList");
const previewAudio = document.getElementById("previewAudio");
const failedSection = document.getElementById("failedSection");
const failedList = document.getElementById("failedList");
const exportFailedBtn = document.getElementById("exportFailedBtn");
const retryFailedBtn = document.getElementById("retryFailedBtn");

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
const toggleLogsBtn = document.getElementById("toggleLogsBtn");
const logsPanel = document.getElementById("logs-panel");

const themeToggleBtn = document.getElementById("themeToggleBtn");
const versionBadge = document.getElementById("versionBadge");
const footerVersion = document.getElementById("footerVersion");
const loadingSkeleton = document.getElementById("loadingSkeleton");
const toastHost = document.getElementById("toastHost");
const srAnnouncer = document.getElementById("srAnnouncer");
const configCard = document.getElementById("configCard");
const configSummary = document.getElementById("configSummary");
const configToggleBtn = document.getElementById("configToggleBtn");
const bottomConfigSummary = document.getElementById("bottomConfigSummary");
const bottomActionBar = document.getElementById("bottomActionBar");

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
  filePreviewRowMap: new Map(),
  fileRowMap: new Map(),
  lastSummary: null,
  abortController: null,
  progressDone: 0,
  progressTotal: 0,
  failedResults: [],
  progressPulseDone: false,
  configCollapsed: false
};

let configController = null;

const toast = createToastManager({ host: toastHost, maxVisible: 3, defaultDuration: 3000 });

function hasGSAP() {
  return typeof window !== "undefined" && Boolean(window.gsap);
}

const iconToolkit = createIconToolkit({
  escapeHtml,
  extIconMap: EXT_ICON_MAP,
  fallbackCdns: LUCIDE_FALLBACK_CDNS
});

const {
  refreshIcons,
  iconMarkup,
  renderExtBadge,
  setButtonContent,
  setStatusIcon
} = iconToolkit;

function syncBusyVisualState() {
  const isLoading = document.body.classList.contains("app-loading");
  const busy = Boolean(state.isBusy || isLoading);
  document.body.classList.toggle("is-busy", busy);
  document.body.setAttribute("aria-busy", busy ? "true" : "false");
}

function setSkeletonLoading(isLoading) {
  document.body.classList.toggle("app-loading", Boolean(isLoading));
  if (loadingSkeleton) loadingSkeleton.setAttribute("aria-hidden", isLoading ? "false" : "true");
  syncBusyVisualState();
}

function isFileDragEvent(event) {
  const types = event?.dataTransfer?.types;
  if (!types) return false;
  for (let i = 0; i < types.length; i += 1) {
    if (String(types[i]).toLowerCase() === "files") return true;
  }
  return false;
}

function decorateStaticActionButtons() {
  const buttons = Array.from(document.querySelectorAll("button[data-icon]"));
  buttons.forEach((button) => {
    if (button === convertBtn || button === cancelBtn || button === themeToggleBtn) return;
    const iconName = button.getAttribute("data-icon") || "";
    const text = button.textContent.trim();
    if (!text) return;
    setButtonContent(button, text, iconName);
  });
  refreshIcons();
}

function animatePageEntrance() {
  const hero = document.querySelector(".hero");
  const cards = document.querySelectorAll(".card");
  slideDown(hero, { duration: 0.6, ease: "power3.out" });
  stagger(cards, { duration: 0.5, stagger: 0.12, ease: "power2.out", delay: 0.05 });
}

function pulseDropZone() {
  if (!hasGSAP() || prefersReducedMotion()) return;
  window.gsap.killTweensOf(dropZone);
  window.gsap.fromTo(
    dropZone,
    { boxShadow: "0 0 0 0 rgba(14,165,233,0.22)" },
    {
      boxShadow: "0 0 0 8px rgba(14,165,233,0)",
      duration: 0.22,
      ease: "power2.out"
    }
  );
}

function animateFileRemove(row, onComplete) {
  if (!row || !hasGSAP() || prefersReducedMotion()) {
    onComplete();
    return;
  }
  window.gsap.to(row, {
    opacity: 0,
    y: -12,
    duration: 0.25,
    ease: "power1.in",
    onComplete
  });
}

function applyMicroInteractions() {
  if (!hasGSAP() || prefersReducedMotion()) return;
  document.addEventListener("pointerdown", (event) => {
    const button = event.target.closest("button:not(:disabled)");
    if (!button) return;
    window.gsap.to(button, { opacity: 0.8, duration: 0.08, yoyo: true, repeat: 1, ease: "power1.out" });
  });
}

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
  if (payload?.detail) {
    const compactDetail = String(payload.detail).replace(/\s+/g, " ").trim();
    if (compactDetail) {
      const brief = compactDetail.length > 120 ? `${compactDetail.slice(0, 120)}...` : compactDetail;
      appendLog("info", `详情：${brief}`);
    }
  }
}

function toastInfo(message) {
  toast.info(message);
}

function toastSuccess(message) {
  toast.success(message);
}

function toastError(message) {
  toast.error(message);
}

async function runWithButtonLoading(button, loadingText, runTask) {
  return withButtonLoading(
    button,
    async () => runTask(),
    {
      setButtonContent,
      loadingText,
      loadingIcon: "loader-circle"
    }
  );
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

function updateVersionBadge(serverVersion = "") {
  const frontendVersion = String(APP_VERSION || "").trim();
  const backendVersion = String(serverVersion || "").trim();
  const frontendIsPlaceholder = !frontendVersion || frontendVersion === "dev";
  const displayVersion = frontendIsPlaceholder && backendVersion ? backendVersion : frontendVersion || "dev";
  const preview = isPreviewVersion(displayVersion);

  if (versionBadge) {
    versionBadge.classList.toggle("preview", preview);
    versionBadge.textContent = displayVersion;
  }

  if (footerVersion) {
    footerVersion.textContent = displayVersion;
  }
}

async function syncVersionFromHealth() {
  try {
    const health = await fetchJson(`/api/health?_ts=${Date.now()}`);
    updateVersionBadge(String(health?.version || "").trim());
  } catch {
    updateVersionBadge();
  }
}

const uploader = createUploaderController({
  state,
  elements: {
    filePreviewList,
    fileSummary,
    clearQueueBtn
  },
  helpers: {
    getExt,
    formatBytes,
    renderExtBadge,
    escapeHtml,
    setHintStatus,
    appendLog,
    refreshIcons
  }
});

function renderGlobalAlert() {
  const issues = [];
  if (state.missingTools.length > 0) issues.push(`缺少运行时文件：${state.missingTools.join("、")}`);
  if (uploader.requiresDb() && !uploader.isDbReady()) issues.push("当前队列包含 KGG 文件，但未检测到可用 KGMusicV3.db。");

  if (issues.length === 0) {
    globalAlert.classList.add("hidden");
    globalAlert.innerHTML = "";
    return;
  }

  globalAlert.classList.remove("hidden");
  globalAlert.innerHTML = issues
    .map(
      (issue) => `
        <div class="alert-item">
          ${iconMarkup("triangle-alert", "alert-icon", true)}
          <span>${escapeHtml(issue)}</span>
        </div>
      `
    )
    .join("");
  refreshIcons();
}

function updateConvertButtonState() {
  const ready =
    !state.isBusy &&
    uploader.pendingCount() > 0 &&
    state.missingTools.length === 0 &&
    outputDirInput.value.trim() &&
    (!uploader.requiresDb() || uploader.isDbReady());

  convertBtn.disabled = !ready;
  setButtonContent(convertBtn, state.isBusy ? "转换中..." : "开始转换", state.isBusy ? "loader-circle" : "play");
  convertBtn.setAttribute("aria-label", state.isBusy ? "转换进行中" : "开始转换");
  cancelBtn.disabled = !state.isBusy;
  setButtonContent(cancelBtn, "取消", "square");
  if (clearQueueBtn) clearQueueBtn.disabled = state.isBusy || uploader.pendingCount() === 0;
  updateBottomActionSummary();
  refreshIcons();
}

function updateBottomActionSummary() {
  if (!bottomConfigSummary) return;
  const outputDir = outputDirInput.value.trim() || "未设置";
  const compactDir = outputDir.length > 38 ? `...${outputDir.slice(-35)}` : outputDir;
  const format = String(outputFormatSelect.value || "copy").toUpperCase();
  const dbText = uploader.requiresDb() ? (uploader.isDbReady() ? "DB已就绪" : "DB未就绪") : "无需DB";
  bottomConfigSummary.textContent = `格式：${format} | 输出：${compactDir} | ${dbText}`;
  bottomConfigSummary.setAttribute("title", `输出目录：${outputDir}`);
}

function updateBottomSafeSpacing() {
  const main = document.querySelector("main");
  if (!main || !bottomActionBar) return;
  const minPadding = 112;
  const barHeight = Math.max(bottomActionBar.offsetHeight || 0, minPadding);
  main.style.paddingBottom = `calc(${barHeight + 24}px + env(safe-area-inset-bottom, 0px))`;
}

function updateDropZoneCompactState() {
  const hasPending = uploader.pendingCount() > 0;
  dropZone.classList.toggle("drop-zone-mini", hasPending);
}

function toggleLogsPanel() {
  if (!logsPanel) return;
  const hidden = logsPanel.classList.toggle("hidden");
  if (toggleLogsBtn) {
    const label = hidden ? "查看转换日志与历史" : "收起转换日志与历史";
    toggleLogsBtn.setAttribute("aria-expanded", hidden ? "false" : "true");
    toggleLogsBtn.setAttribute("aria-label", label);
  }
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
  syncBusyVisualState();
  updateConvertButtonState();
}

function queueChanged() {
  uploader.renderFilePreview();
  uploader.updateFileSummary();
  updateDropZoneCompactState();
  renderGlobalAlert();
  updateConvertButtonState();
  updateBottomActionSummary();
  updateBottomSafeSpacing();
  if (configController) configController.updateConfigAutoCollapse();
}

function clearQueueAction() {
  uploader.clearQueue();
  queueChanged();
  appendLog("info", "已清空待转换队列。");
  toastInfo("已清空待转换队列");
}

configController = createConfigController({
  state,
  elements: {
    outputDirInput,
    dbPathInput,
    outputFormatSelect,
    mp3QualitySelect,
    mp3QualityWrap,
    concurrencySelect,
    dbStatus,
    runtimeStatus,
    themeToggleBtn,
    configCard,
    configSummary,
    configToggleBtn
  },
  storage: {
    storageKeys: STORAGE_KEYS,
    themeKey: THEME_KEY,
    configCollapsedKey: CONFIG_COLLAPSED_KEY
  },
  helpers: {
    fetchJson,
    appendLog,
    setHintStatus,
    setButtonContent,
    refreshIcons,
    toastSuccess,
    toastError,
    toastInfo
  },
  callbacks: {
    requiresDb: () => uploader.requiresDb(),
    isDbReady: () => uploader.isDbReady(),
    onStateChanged: queueChanged
  }
});

const a11yAnnouncer = createA11yAnnouncer({ region: srAnnouncer });

const historyController = createHistoryController({
  storageKey: HISTORY_KEY,
  panel: historyPanel,
  escapeHtml,
  formatDuration,
  appendLog,
  toastInfo,
  onRestore: (_, historyItem) => {
    const outputDir = String(historyItem?.outputDir || "").trim();
    const outputFormat = String(historyItem?.outputFormat || "").trim().toLowerCase();
    if (outputDir) outputDirInput.value = outputDir;

    const formatExists = Array.from(outputFormatSelect.options).some((opt) => opt.value === outputFormat);
    if (formatExists) outputFormatSelect.value = outputFormat;

    configController.savePreferences();
    configController.updateMp3QualityVisibility();
    updateConvertButtonState();
    configController.updateConfigAutoCollapse();

    appendLog("info", `已恢复历史配置：输出目录 ${outputDir || "未记录"}，格式 ${String(outputFormatSelect.value).toUpperCase()}`);
    toastInfo("已从历史记录恢复配置");
  }
});

const converter = createConverterController({
  state,
  elements: {
    progressSection,
    totalProgressBar,
    totalProgressText,
    progressStatus,
    progressETA,
    fileProgressList,
    resultDashboard,
    statSuccess,
    statFailed,
    statDuration,
    successSection,
    successList,
    previewAudio,
    failedSection,
    failedList,
    retryFailedBtn,
    outputDirInput,
    dbPathInput,
    outputFormatSelect,
    mp3QualitySelect,
    concurrencySelect
  },
  helpers: {
    hasGSAP,
    prefersReducedMotion,
    fadeIn,
    readSseStream,
    iconMarkup,
    setStatusIcon,
    refreshIcons,
    escapeHtml,
    formatDuration,
    appendLog,
    appendPayloadError,
    toastSuccess,
    toastInfo,
    confirmDestructive
  },
  callbacks: {
    getPendingItems: () => uploader.getPendingItems(),
    requiresDb: () => uploader.requiresDb(),
    isDbReady: () => uploader.isDbReady(),
    getSelectedFiles: () => state.selectedFiles,
    getPathQueue: () => state.pathQueue,
    setBusy,
    onQueueChanged: queueChanged,
    clearQueue: () => uploader.clearQueue(),
    queueFailedResults: (items) => uploader.queueFailedResults(items),
    queueRetrySnapshotItems: (items) => uploader.queueRetrySnapshotItems(items),
    announceCompletion: a11yAnnouncer.announce,
    onConvertComplete: (summary) => {
      historyController.append(summary, {
        outputDir: outputDirInput.value.trim(),
        outputFormat: outputFormatSelect.value
      });
    }
  }
});

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
  renderExtBadge,
  escapeHtml,
  iconMarkup,
  refreshIcons,
  copyTextToClipboard,
  onQueueChanged: queueChanged,
  pendingCount: () => uploader.pendingCount()
});

const updateController = createUpdateController({
  appVersion: APP_VERSION,
  updateBannerHost,
  parseVersion,
  shouldNotifyUpdate,
  iconMarkup,
  escapeHtml,
  refreshIcons,
  hasGSAP,
  prefersReducedMotion,
  slideDown
});

const dragDropController = createDragDropController({
  dropZone,
  isFileDragEvent,
  hasGSAP,
  prefersReducedMotion,
  pulseDropZone,
  onFilesDropped: async (droppedFiles) => {
    uploader.mergeFiles(droppedFiles);
    appendLog("info", `拖放已加入 ${droppedFiles.length} 个文件候选。`);
    queueChanged();
  }
});

function exportLogs() {
  const lines = Array.from(logBox.querySelectorAll(".log-line")).map((line) => line.textContent);
  const blob = new Blob([`\uFEFF${lines.join("\n")}`], { type: "text/plain;charset=utf-8" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = `转换日志_${new Date().toISOString().replace(/[:.]/g, "-")}.txt`;
  a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 10000);
  toastSuccess("日志已导出");
}

async function copyTextToClipboard(text, okMessage) {
  try {
    await navigator.clipboard.writeText(text);
    appendLog("success", okMessage);
    toastSuccess(okMessage);
  } catch {
    appendLog("error", "复制失败，请检查剪贴板权限。");
    toastError("复制失败，请检查剪贴板权限");
  }
}

function bindEvents() {
  pickFilesBtn.addEventListener("click", () => fileInput.click());
  fileInput.addEventListener("change", () => {
    uploader.mergeFiles(fileInput.files);
    fileInput.value = "";
    queueChanged();
  });
  dragDropController.bind();

  filePreviewList.addEventListener("click", (event) => {
    const btn = event.target.closest(".btn-remove");
    if (!btn) return;
    const source = btn.getAttribute("data-source");
    const index = Number.parseInt(btn.getAttribute("data-index"), 10);
    if (!Number.isFinite(index)) return;
    const removeAction = () => {
      uploader.removeAt(source, index);
      queueChanged();
    };
    animateFileRemove(btn.closest(".file-preview-item"), removeAction);
  });

  pickDirBtn.addEventListener("click", () => {
    runWithButtonLoading(pickDirBtn, "选择中...", configController.pickOutputDir);
  });
  openDirBtn.addEventListener("click", configController.openOutputFolder);
  openOutputBtn.addEventListener("click", configController.openOutputFolder);
  openResultDirBtn.addEventListener("click", configController.openOutputFolder);
  pickDbBtn.addEventListener("click", () => {
    runWithButtonLoading(pickDbBtn, "选择中...", configController.pickDbFile);
  });
  redetectDbBtn.addEventListener("click", () => {
    runWithButtonLoading(redetectDbBtn, "检测中...", configController.redetectDb);
  });

  convertBtn.addEventListener("click", converter.startConvert);
  cancelBtn.addEventListener("click", converter.cancelConvert);
  exportLogBtn.addEventListener("click", exportLogs);
  exportFailedBtn.addEventListener("click", converter.exportFailedList);

  if (retryFailedBtn) {
    retryFailedBtn.addEventListener("click", () => converter.retryFailedFiles(state.failedResults));
  }
  if (clearQueueBtn) {
    clearQueueBtn.addEventListener("click", () => {
      const ok = confirmDestructive("确定清空全部待转换文件吗？");
      if (!ok) return;
      clearQueueAction();
    });
  }

  outputDirInput.addEventListener("input", () => {
    configController.savePreferences();
    updateConvertButtonState();
    configController.updateConfigAutoCollapse();
  });
  dbPathInput.addEventListener("input", () => {
    configController.savePreferences();
    configController.scheduleDbValidation();
    configController.updateConfigAutoCollapse();
  });
  outputFormatSelect.addEventListener("change", () => {
    configController.savePreferences();
    configController.updateMp3QualityVisibility();
    configController.updateConfigAutoCollapse();
  });
  mp3QualitySelect.addEventListener("change", configController.savePreferences);
  concurrencySelect.addEventListener("change", () => {
    configController.savePreferences();
    configController.updateConfigAutoCollapse();
  });

  clearHistoryBtn.addEventListener("click", () => {
    const confirmed = confirmDestructive("确定清空全部转换历史吗？此操作不可撤销。");
    if (!confirmed) return;
    historyController.clear();
    appendLog("info", "历史记录已清空。");
    toastInfo("历史记录已清空");
  });

  themeToggleBtn.addEventListener("click", configController.toggleTheme);
  if (toggleLogsBtn) toggleLogsBtn.addEventListener("click", toggleLogsPanel);

  pickFoldersBtn.addEventListener("click", () => {
    runWithButtonLoading(pickFoldersBtn, "选择中...", scanner.pickFolderForScan);
  });
  selectedFolders.addEventListener("click", (event) => {
    const btn = event.target.closest(".tag-remove");
    if (!btn) return;
    const index = Number.parseInt(btn.getAttribute("data-index"), 10);
    scanner.removeFolder(index);
  });

  extFilter.addEventListener("change", scanner.handleExtFilterChange);
  scanBtn.addEventListener("click", () => {
    runWithButtonLoading(scanBtn, "扫描中...", scanner.startScanFolders);
  });
  copyNamesBtn.addEventListener("click", scanner.copyNames);
  copyPathsBtn.addEventListener("click", scanner.copyPaths);
  exportCsvBtn.addEventListener("click", scanner.exportCsv);
  selectAllForConvert.addEventListener("click", scanner.addScanFilesToQueue);

  failedList.addEventListener("click", (event) => {
    const btn = event.target.closest(".retry-single-btn");
    if (!btn) return;
    const index = Number.parseInt(btn.getAttribute("data-index"), 10);
    converter.retryFailedAt(index);
  });
  if (successList) {
    successList.addEventListener("click", converter.handleSuccessListClick);
  }

  if (configToggleBtn) {
    configToggleBtn.addEventListener("click", () => {
      configController.setConfigCollapsed(!state.configCollapsed, true);
    });
  }

  window.addEventListener("resize", updateBottomSafeSpacing);
  window.addEventListener("orientationchange", updateBottomSafeSpacing);
}

(async function init() {
  setSkeletonLoading(true);
  updateVersionBadge();
  configController.initTheme();
  decorateStaticActionButtons();
  applyMicroInteractions();
  configController.loadPreferences();
  historyController.load();
  historyController.render();
  bindEvents();
  if (toggleLogsBtn && logsPanel) {
    toggleLogsBtn.setAttribute("aria-expanded", logsPanel.classList.contains("hidden") ? "false" : "true");
    toggleLogsBtn.setAttribute("aria-controls", "logs-panel");
  }
  scanner.renderFolderTags();
  configController.updateMp3QualityVisibility();
  configController.updateConfigAutoCollapse();
  uploader.renderFilePreview();
  uploader.updateFileSummary();
  updateDropZoneCompactState();
  updateBottomSafeSpacing();
  if (retryFailedBtn) retryFailedBtn.disabled = true;
  setBusy(false);

  await configController.loadConfig();
  await syncVersionFromHealth();
  setSkeletonLoading(false);
  if (dbPathInput.value.trim()) configController.scheduleDbValidation();
  updateController.checkForUpdates();
  animatePageEntrance();
  refreshIcons();

  appendLog("info", "页面已就绪。");
})();
