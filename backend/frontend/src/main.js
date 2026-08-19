const elements = {
  workspace: document.getElementById("workspace"),
  versionBadge: document.getElementById("versionBadge"),
  runtimeBadge: document.getElementById("runtimeBadge"),
  runtimeMessage: document.getElementById("runtimeMessage"),
  checkUpdateButton: document.getElementById("checkUpdateButton"),
  themeToggleButton: document.getElementById("themeToggleButton"),
  updateNotice: document.getElementById("updateNotice"),
  updateNoticeTitle: document.getElementById("updateNoticeTitle"),
  updateStatus: document.getElementById("updateStatus"),
  openUpdateButton: document.getElementById("openUpdateButton"),
  openReleasePageButton: document.getElementById("openReleasePageButton"),
  ignoreUpdateButton: document.getElementById("ignoreUpdateButton"),
  dismissUpdateButton: document.getElementById("dismissUpdateButton"),
  pickFilesButton: document.getElementById("pickFilesButton"),
  scanFolderButton: document.getElementById("scanFolderButton"),
  scanRecursive: document.getElementById("scanRecursive"),
  scanHelp: document.getElementById("scanHelp"),
  scanError: document.getElementById("scanError"),
  clearFilesButton: document.getElementById("clearFilesButton"),
  emptyQueue: document.getElementById("emptyQueue"),
  queueSummary: document.getElementById("queueSummary"),
  fileList: document.getElementById("fileList"),
  fileError: document.getElementById("fileError"),
  previewPanel: document.getElementById("previewPanel"),
  previewFileName: document.getElementById("previewFileName"),
  previewAudio: document.getElementById("previewAudio"),
  formatSelect: document.getElementById("formatSelect"),
  qualityField: document.getElementById("qualityField"),
  qualitySelect: document.getElementById("qualitySelect"),
  concurrencySelect: document.getElementById("concurrencySelect"),
  outputDirectory: document.getElementById("outputDirectory"),
  pickOutputButton: document.getElementById("pickOutputButton"),
  openConfiguredOutputButton: document.getElementById("openConfiguredOutputButton"),
  outputError: document.getElementById("outputError"),
  databasePath: document.getElementById("databasePath"),
  pickDatabaseButton: document.getElementById("pickDatabaseButton"),
  redetectDatabaseButton: document.getElementById("redetectDatabaseButton"),
  databaseError: document.getElementById("databaseError"),
  convertButton: document.getElementById("convertButton"),
  cancelButton: document.getElementById("cancelButton"),
  convertHelp: document.getElementById("convertHelp"),
  conversionError: document.getElementById("conversionError"),
  progressPanel: document.getElementById("progressPanel"),
  progressStatus: document.getElementById("progressStatus"),
  progressPercent: document.getElementById("progressPercent"),
  totalProgress: document.getElementById("totalProgress"),
  totalProgressTrack: document.getElementById("totalProgressTrack"),
  resultSummary: document.getElementById("resultSummary"),
  resultSummaryText: document.getElementById("resultSummaryText"),
  statSuccess: document.getElementById("statSuccess"),
  statFailed: document.getElementById("statFailed"),
  statDuration: document.getElementById("statDuration"),
  retryFailedButton: document.getElementById("retryFailedButton"),
  exportFailedButton: document.getElementById("exportFailedButton"),
  openOutputButton: document.getElementById("openOutputButton"),
  historyToggleButton: document.getElementById("historyToggleButton"),
  clearHistoryButton: document.getElementById("clearHistoryButton"),
  historyEmpty: document.getElementById("historyEmpty"),
  historyList: document.getElementById("historyList"),
  addAdvancedFolderButton: document.getElementById("addAdvancedFolderButton"),
  clearAdvancedFoldersButton: document.getElementById("clearAdvancedFoldersButton"),
  advancedFolderList: document.getElementById("advancedFolderList"),
  advancedRecursive: document.getElementById("advancedRecursive"),
  advancedFilter: document.getElementById("advancedFilter"),
  customFilterField: document.getElementById("customFilterField"),
  customFilterInput: document.getElementById("customFilterInput"),
  runAdvancedScanButton: document.getElementById("runAdvancedScanButton"),
  advancedScanStatus: document.getElementById("advancedScanStatus"),
  advancedScanError: document.getElementById("advancedScanError"),
  advancedResult: document.getElementById("advancedResult"),
  advancedResultSummary: document.getElementById("advancedResultSummary"),
  advancedResultList: document.getElementById("advancedResultList"),
  copyAdvancedNamesButton: document.getElementById("copyAdvancedNamesButton"),
  copyAdvancedPathsButton: document.getElementById("copyAdvancedPathsButton"),
  exportAdvancedCsvButton: document.getElementById("exportAdvancedCsvButton"),
  addAdvancedQueueButton: document.getElementById("addAdvancedQueueButton"),
  exportDiagnosticButton: document.getElementById("exportDiagnosticButton"),
  diagnosticStatus: document.getElementById("diagnosticStatus"),
  diagnosticList: document.getElementById("diagnosticList"),
  refreshRuntimeCacheButton: document.getElementById("refreshRuntimeCacheButton"),
  clearRuntimeCacheButton: document.getElementById("clearRuntimeCacheButton"),
  runtimeCacheStatus: document.getElementById("runtimeCacheStatus"),
  contextMenu: document.getElementById("contextMenu"),
  githubLinkButton: document.getElementById("githubLinkButton"),
  findMusicButton: document.getElementById("findMusicButton"),
  findMusicPanel: document.getElementById("findMusicPanel"),
  findMusicError: document.getElementById("findMusicError"),
  findMusicStatus: document.getElementById("findMusicStatus"),
  findMusicResult: document.getElementById("findMusicResult"),
  findMusicSelectAll: document.getElementById("findMusicSelectAll"),
  findMusicSummary: document.getElementById("findMusicSummary"),
  findMusicList: document.getElementById("findMusicList"),
  findMusicAddButton: document.getElementById("findMusicAddButton"),
  findMusicClearButton: document.getElementById("findMusicClearButton"),
  findMusicCollapseButton: document.getElementById("findMusicCollapseButton"),
  firstRunOverlay: document.getElementById("firstRunOverlay"),
  firstRunConfirm: document.getElementById("firstRunConfirm")
};

const state = {
  files: [],
  fileResults: new Map(),
  runtimeReady: false,
  isBusy: false,
  isScanning: false,
  isAdvancedScanning: false,
  isCheckingUpdate: false,
  cancelRequested: false,
  activeTaskId: "",
  taskStartedAt: 0,
  defaultConcurrency: 1,
  maxConcurrency: 1,
  dbPath: "",
  lastOutputDir: "",
  currentVersion: "",
  lastUpdateURL: "",
  lastUpdateRelease: null,
  historyExpanded: false,
  advancedFolders: [],
  advancedFiles: [],
  diagnostics: [],
  selectedPath: "",
  contextMenuPath: "",
  isFindingMusic: false,
  findMusicGroups: [],
  findMusicChecked: new Set(),
  findMusicWarning: "",
  isManagingCache: false,
  runtimeCacheInfo: null,
  isInstallingUpdate: false
};

const historyStorageKey = "kugo-desktop-history-v1";
const preferencesStorageKey = "kugo-desktop-preferences-v1";
const updateCheckStorageKey = "kugo-desktop-update-check-v1";
const ignoredUpdateStorageKey = "kugo-desktop-ignored-update-v1";
const themeStorageKey = "kugo-desktop-theme";
const disclaimerStorageKey = "kugo-desktop-disclaimer-v1";
const updateCheckInterval = 24 * 60 * 60 * 1000;
const encryptedExtensions = new Set([
  ".kgg", ".kgm", ".kgma", ".vpr", ".ncm", ".kwm",
  ".qmc0", ".qmc2", ".qmc3", ".qmc4", ".qmc6", ".qmc8", ".qmcflac", ".qmcogg", ".tkm"
]);

const extBadgeStyles = {
  ".kgg": ["ext-kgg", "i-lock"],
  ".kgm": ["ext-kgm", "i-music"],
  ".kgma": ["ext-kgma", "i-music"],
  ".vpr": ["ext-vpr", "i-file-audio"],
  ".ncm": ["ext-ncm", "i-disc"],
  ".kwm": ["ext-kwm", "i-music"],
  ".qmc0": ["ext-qmc", "i-lock"],
  ".qmc2": ["ext-qmc", "i-lock"],
  ".qmc3": ["ext-qmc", "i-lock"],
  ".qmc4": ["ext-qmc", "i-lock"],
  ".qmc6": ["ext-qmc", "i-lock"],
  ".qmc8": ["ext-qmc", "i-lock"],
  ".qmcflac": ["ext-qmc", "i-lock"],
  ".qmcogg": ["ext-qmc", "i-lock"],
  ".tkm": ["ext-qmc", "i-lock"]
};

const phaseLabels = {
  prepare: "准备文件",
  decrypt: "解密音频",
  transcode: "生成输出"
};

function desktopApi() {
  return window.go?.main?.App;
}

/* ───── 主题：默认跟随系统，可手动覆盖并记忆 ───── */
function effectiveThemeIsDark() {
  const explicit = document.documentElement.dataset.theme;
  if (explicit === "dark") return true;
  if (explicit === "light") return false;
  return Boolean(window.matchMedia?.("(prefers-color-scheme: dark)").matches);
}

function syncThemeToggle() {
  if (!elements.themeToggleButton) return;
  const dark = effectiveThemeIsDark();
  const label = dark ? "切换到浅色模式" : "切换到深色模式";
  elements.themeToggleButton.setAttribute("aria-pressed", dark ? "true" : "false");
  elements.themeToggleButton.setAttribute("aria-label", label);
  elements.themeToggleButton.title = label;
}

function initTheme() {
  let stored = "";
  try {
    stored = localStorage.getItem(themeStorageKey) || "";
  } catch {
    stored = "";
  }
  if (stored === "light" || stored === "dark") document.documentElement.dataset.theme = stored;
  syncThemeToggle();
  window.matchMedia?.("(prefers-color-scheme: dark)").addEventListener?.("change", syncThemeToggle);
}

function setButtonText(button, text) {
  const label = button.querySelector(".btn-label");
  if (label) label.textContent = text;
  else button.textContent = text;
}

function pathKey(path) {
  return String(path || "").trim().toLowerCase();
}

function formatBytes(value) {
  const bytes = Number(value);
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const display = bytes / 1024 ** index;
  return `${display.toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}

function formatDuration(value) {
  const milliseconds = Math.max(0, Number(value) || 0);
  if (milliseconds < 1000) return `${Math.round(milliseconds)} ms`;
  const seconds = milliseconds / 1000;
  if (seconds < 60) return `${seconds.toFixed(seconds < 10 ? 1 : 0)} 秒`;
  const minutes = Math.floor(seconds / 60);
  return `${minutes} 分 ${Math.round(seconds % 60)} 秒`;
}

function errorText(error) {
  if (typeof error === "string") return error;
  if (error?.message) return String(error.message);
  return String(error || "发生未知错误");
}

function recordDiagnostic(level, message, detail = "") {
  state.diagnostics.push({
    time: new Date().toISOString(),
    level: String(level || "info"),
    message: String(message || ""),
    detail: String(detail || "")
  });
  if (state.diagnostics.length > 200) state.diagnostics.splice(0, state.diagnostics.length - 200);
  renderDiagnostics();
}

function renderDiagnostics() {
  elements.diagnosticList.replaceChildren();
  state.diagnostics.slice(-50).forEach((entry) => {
    const item = document.createElement("li");
    item.dataset.level = entry.level;
    const time = document.createElement("time");
    time.dateTime = entry.time;
    time.textContent = new Date(entry.time).toLocaleTimeString("zh-CN", { hour12: false });
    const text = document.createElement("span");
    text.textContent = `${entry.message}${entry.detail ? ` · ${entry.detail}` : ""}`;
    item.append(time, text);
    elements.diagnosticList.append(item);
  });
}

function setInlineError(element, input, message = "") {
  element.textContent = message;
  element.hidden = !message;
  if (input) input.setAttribute("aria-invalid", message ? "true" : "false");
}

function showFileError(message = "") {
  setInlineError(elements.fileError, null, message);
}

function showScanError(message = "") {
  setInlineError(elements.scanError, null, message);
}

function showAdvancedScanError(message = "") {
  setInlineError(elements.advancedScanError, null, message);
}

function showOutputError(message = "") {
  setInlineError(elements.outputError, elements.outputDirectory, message);
}

function showDatabaseError(message = "") {
  setInlineError(elements.databaseError, elements.databasePath, message);
}

function showConversionError(message = "") {
  setInlineError(elements.conversionError, null, message);
}

function hasKGGFiles(files = state.files) {
  return files.some((file) => String(file.name || "").toLowerCase().endsWith(".kgg"));
}

function conversionDisabledReason() {
  if (!state.runtimeReady) return "FFmpeg 运行时尚未就绪。";
  if (state.isScanning || state.isAdvancedScanning) return "正在扫描文件夹，请稍候。";
  if (state.files.length === 0) return "请先选择至少一个加密音频文件。";
  if (!elements.outputDirectory.value.trim()) return "请先选择输出目录。";
  if (hasKGGFiles() && !state.dbPath) return "队列中包含 KGG 文件，请先选择 KGMusicV3.db。";
  return "";
}

function readPreferences() {
  try {
    const parsed = JSON.parse(localStorage.getItem(preferencesStorageKey) || "{}");
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function savePreferences() {
  try {
    localStorage.setItem(preferencesStorageKey, JSON.stringify({
      outputDir: elements.outputDirectory.value.trim(),
      outputFormat: elements.formatSelect.value,
      mp3Quality: Number(elements.qualitySelect.value),
      concurrency: Number(elements.concurrencySelect.value) || state.defaultConcurrency,
      dbPath: state.dbPath
    }));
  } catch {
    recordDiagnostic("warning", "设置保存失败", "WebView 存储不可用");
  }
}

function populateConcurrencySelect(maximum, selected) {
  const max = Math.max(1, Math.min(64, Number(maximum) || 1));
  const value = Math.max(1, Math.min(max, Number(selected) || 1));
  elements.concurrencySelect.replaceChildren();
  for (let concurrency = 1; concurrency <= max; concurrency += 1) {
    const option = document.createElement("option");
    option.value = String(concurrency);
    option.textContent = concurrency === 1 ? "1（最稳妥）" : String(concurrency);
    elements.concurrencySelect.append(option);
  }
  elements.concurrencySelect.value = String(value);
}

function syncFormatSettings() {
  const isMP3 = elements.formatSelect.value === "mp3";
  elements.qualityField.hidden = !isMP3;
  elements.qualitySelect.disabled = state.isBusy || state.isScanning || state.isAdvancedScanning || !isMP3;
}

function failedEntries() {
  return state.files
    .map((file) => ({ file, result: resultForFile(file) }))
    .filter(({ result }) => result?.status === "error" && result.error?.code !== "ERR_CANCELLED");
}

function syncControls() {
  const disabledReason = conversionDisabledReason();
  const inputLocked = state.isBusy || state.isScanning || state.isAdvancedScanning || state.isFindingMusic;
  const hasHistory = readHistory().length > 0;
  const failures = failedEntries().length;
  elements.workspace.setAttribute("aria-busy", inputLocked ? "true" : "false");
  elements.convertButton.hidden = state.isBusy;
  elements.cancelButton.hidden = !state.isBusy;
  elements.convertButton.disabled = state.isBusy || Boolean(disabledReason);
  elements.cancelButton.disabled = !state.isBusy || state.cancelRequested;
  elements.convertHelp.textContent = state.isBusy
    ? state.cancelRequested
      ? "正在停止当前任务，请等待已启动的文件处理结束。"
      : "转换期间可以取消；已生成的成功文件会保留。"
    : disabledReason || "所有文件将直接在本机处理，不会发送到网络。";
  elements.pickFilesButton.disabled = inputLocked;
  elements.scanFolderButton.disabled = inputLocked;
  elements.findMusicButton.disabled = inputLocked;
  setButtonText(elements.findMusicButton, state.isFindingMusic ? "正在检测…" : "查找本机音乐");
  elements.scanRecursive.disabled = inputLocked;
  elements.clearFilesButton.disabled = inputLocked;
  elements.clearFilesButton.hidden = state.files.length === 0;
  elements.pickOutputButton.disabled = inputLocked;
  elements.openConfiguredOutputButton.disabled = inputLocked || !elements.outputDirectory.value.trim();
  elements.pickDatabaseButton.disabled = inputLocked;
  elements.redetectDatabaseButton.disabled = inputLocked;
  elements.formatSelect.disabled = inputLocked;
  elements.concurrencySelect.disabled = inputLocked;
  elements.checkUpdateButton.disabled = state.isCheckingUpdate;
  setButtonText(elements.checkUpdateButton, state.isCheckingUpdate ? "正在检查…" : "检查更新");
  elements.openUpdateButton.disabled = state.isInstallingUpdate || state.isBusy;
  elements.openReleasePageButton.disabled = state.isInstallingUpdate;
  setButtonText(elements.openUpdateButton, state.isInstallingUpdate ? "正在下载…" : "下载并安装");
  elements.retryFailedButton.hidden = failures === 0;
  elements.retryFailedButton.disabled = inputLocked || failures === 0;
  elements.exportFailedButton.hidden = failures === 0;
  elements.exportFailedButton.disabled = inputLocked || failures === 0;
  elements.clearHistoryButton.hidden = !hasHistory;
  elements.clearHistoryButton.disabled = inputLocked || !hasHistory;
  elements.historyToggleButton.disabled = inputLocked;
  elements.historyList.querySelectorAll("button").forEach((button) => { button.disabled = inputLocked; });
  elements.addAdvancedFolderButton.disabled = inputLocked;
  elements.clearAdvancedFoldersButton.disabled = inputLocked;
  elements.advancedRecursive.disabled = inputLocked;
  elements.advancedFilter.disabled = inputLocked;
  elements.customFilterInput.disabled = inputLocked;
  elements.runAdvancedScanButton.disabled = inputLocked || state.advancedFolders.length === 0;
  setButtonText(elements.runAdvancedScanButton, state.isAdvancedScanning ? "正在扫描…" : "开始扫描");
  elements.copyAdvancedNamesButton.disabled = inputLocked || state.advancedFiles.length === 0;
  elements.copyAdvancedPathsButton.disabled = inputLocked || state.advancedFiles.length === 0;
  elements.exportAdvancedCsvButton.disabled = inputLocked || state.advancedFiles.length === 0;
  elements.addAdvancedQueueButton.disabled = inputLocked || !state.advancedFiles.some(isConvertibleScanFile);
  elements.exportDiagnosticButton.disabled = state.isAdvancedScanning;
  elements.refreshRuntimeCacheButton.disabled = state.isManagingCache || state.isBusy;
  elements.clearRuntimeCacheButton.disabled = state.isManagingCache || state.isBusy || !state.runtimeCacheInfo?.reclaimableItems;
  setButtonText(elements.clearRuntimeCacheButton, state.isManagingCache ? "正在处理…" : "清理旧缓存");
  syncFormatSettings();
}

function resultForFile(file) {
  return state.fileResults.get(pathKey(file.path)) || state.fileResults.get(pathKey(file.name));
}

function resetPreview() {
  elements.previewAudio.pause();
  elements.previewAudio.removeAttribute("src");
  elements.previewAudio.removeAttribute("data-output");
  elements.previewAudio.load();
  elements.previewFileName.textContent = "";
  elements.previewPanel.hidden = true;
}

function addFiles(files) {
  const byPath = new Map(state.files.map((file) => [pathKey(file.path), file]));
  let added = 0;
  let duplicate = 0;
  let blocked = 0;
  for (const file of Array.isArray(files) ? files : []) {
    const key = pathKey(file?.path);
    if (!key) continue;
    if (byPath.has(key)) {
      duplicate += 1;
      continue;
    }
    if (byPath.size >= 500) {
      blocked += 1;
      continue;
    }
    byPath.set(key, file);
    added += 1;
  }
  state.files = Array.from(byPath.values());
  if (added > 0) {
    state.fileResults.clear();
    elements.progressPanel.hidden = true;
    elements.resultSummary.hidden = true;
    resetPreview();
    recordDiagnostic("info", "文件加入队列", `新增 ${added}，重复 ${duplicate}，超限 ${blocked}`);
  }
  renderFiles();
  return { added, duplicate, blocked };
}

function removeQueuedFile(file) {
  if (state.isBusy || state.isScanning || state.isAdvancedScanning) return;
  const key = pathKey(file.path);
  const existingResult = resultForFile(file);
  state.files = state.files.filter((item) => pathKey(item.path) !== key);
  state.fileResults.delete(key);
  if (state.selectedPath === key) state.selectedPath = "";
  if (state.contextMenuPath === key) closeContextMenu();
  if (elements.previewAudio.dataset.output === existingResult?.output) resetPreview();
  if (state.files.length === 0) elements.progressPanel.hidden = true;
  recordDiagnostic("info", "从队列移除文件", file.name || file.path);
  renderFiles();
}

function createFileAction(label, ariaLabel, onClick, danger = false) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = `button ${danger ? "button-danger" : "button-secondary"} button-inline`;
  button.textContent = label;
  button.setAttribute("aria-label", ariaLabel);
  button.disabled = state.isBusy || state.isScanning || state.isAdvancedScanning;
  button.addEventListener("click", onClick);
  return button;
}

function createExtBadge(fileName) {
  const ext = `.${String(fileName || "").split(".").pop().toLowerCase()}`;
  const [colorClass, iconId] = extBadgeStyles[ext] || ["ext-default", "i-file"];
  const badge = document.createElement("span");
  badge.className = `ext-badge ${colorClass}`;
  badge.setAttribute("aria-hidden", "true");
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("class", "icon");
  svg.setAttribute("aria-hidden", "true");
  const use = document.createElementNS("http://www.w3.org/2000/svg", "use");
  use.setAttribute("href", `#${iconId}`);
  svg.append(use);
  badge.append(svg, document.createTextNode(ext.replace(".", "").toUpperCase()));
  return badge;
}

function selectQueueFile(file) {
  const key = file ? pathKey(file.path) : "";
  if (state.selectedPath === key) return;
  state.selectedPath = key;
  elements.fileList.querySelectorAll(".file-row").forEach((row) => {
    row.classList.toggle("selected", row.dataset.pathKey === key && key !== "");
  });
}

function selectedQueueFile() {
  if (!state.selectedPath) return null;
  return state.files.find((file) => pathKey(file.path) === state.selectedPath) || null;
}

/* ───── 队列右键菜单 ───── */
function closeContextMenu() {
  if (elements.contextMenu.hidden) return;
  elements.contextMenu.hidden = true;
  state.contextMenuPath = "";
}

function contextMenuAction(iconId, label, onClick, danger = false) {
  const item = document.createElement("button");
  item.type = "button";
  item.className = `context-menu-item${danger ? " danger" : ""}`;
  item.setAttribute("role", "menuitem");
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("class", "icon");
  svg.setAttribute("aria-hidden", "true");
  const use = document.createElementNS("http://www.w3.org/2000/svg", "use");
  use.setAttribute("href", `#${iconId}`);
  svg.append(use);
  item.append(svg, document.createTextNode(label));
  item.addEventListener("click", () => {
    closeContextMenu();
    onClick();
  });
  return item;
}

function copyQueueText(text, fallbackMessage) {
  if (window.runtime?.ClipboardSetText) {
    window.runtime.ClipboardSetText(text).catch(() => {});
  } else if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).catch(() => {});
  }
  recordDiagnostic("info", fallbackMessage);
}

function openContextMenu(file, x, y) {
  const result = resultForFile(file);
  const menu = elements.contextMenu;
  menu.replaceChildren();
  if (result?.status === "ok" && result.output) {
    menu.append(
      contextMenuAction("i-play", "试听转换结果", () => previewOutput(result.output, file.name)),
      contextMenuAction("i-folder-open", "在资源管理器中定位", () => locateOutput(result.output))
    );
  }
  if (result?.status === "error" && result.error?.code !== "ERR_CANCELLED") {
    menu.append(contextMenuAction("i-rotate-cw", "重试此文件", () => startConversion([file], true)));
  }
  if (menu.childElementCount > 0) {
    const separator = document.createElement("div");
    separator.className = "context-menu-separator";
    menu.append(separator);
  }
  menu.append(
    contextMenuAction("i-copy", "复制文件名", () => copyQueueText(file.name, "已复制文件名")),
    contextMenuAction("i-file", "复制完整路径", () => copyQueueText(file.path, "已复制文件路径")),
    contextMenuAction("i-trash-2", "从队列移除", () => removeQueuedFile(file), true)
  );
  menu.hidden = false;
  const { innerWidth, innerHeight } = window;
  const rect = menu.getBoundingClientRect();
  menu.style.left = `${Math.max(8, Math.min(x, innerWidth - rect.width - 8))}px`;
  menu.style.top = `${Math.max(8, Math.min(y, innerHeight - rect.height - 8))}px`;
  state.contextMenuPath = pathKey(file.path);
  menu.querySelector(".context-menu-item")?.focus();
}

function renderFiles() {
  const hasFiles = state.files.length > 0;
  elements.emptyQueue.hidden = hasFiles;
  elements.queueSummary.hidden = !hasFiles;
  elements.fileList.hidden = !hasFiles;
  elements.fileList.replaceChildren();
  if (!hasFiles) {
    syncControls();
    return;
  }
  const totalBytes = state.files.reduce((sum, file) => sum + Number(file.size || 0), 0);
  const successes = state.files.filter((file) => resultForFile(file)?.status === "ok").length;
  const failures = state.files.filter((file) => resultForFile(file)?.status === "error").length;
  const resultText = successes || failures ? ` · 成功 ${successes} · 失败或取消 ${failures}` : "";
  elements.queueSummary.textContent = `已选择 ${state.files.length} 个文件 · ${formatBytes(totalBytes)}${resultText}`;

  state.files.forEach((file) => {
    const result = resultForFile(file);
    const item = document.createElement("li");
    item.className = "file-row";
    item.dataset.pathKey = pathKey(file.path);
    if (state.selectedPath === item.dataset.pathKey) item.classList.add("selected");
    item.addEventListener("click", () => selectQueueFile(file));
    item.addEventListener("dblclick", () => {
      if (result?.status === "ok" && result.output) previewOutput(result.output, file.name);
    });
    item.addEventListener("contextmenu", (event) => {
      event.preventDefault();
      selectQueueFile(file);
      openContextMenu(file, event.clientX, event.clientY);
    });
    const main = document.createElement("span");
    main.className = "file-row-main";
    const name = document.createElement("span");
    name.className = "file-name";
    name.textContent = file.name;
    name.title = file.path;
    const title = document.createElement("span");
    title.className = "file-title";
    title.append(createExtBadge(file.name), name);
    const size = document.createElement("span");
    size.className = "file-size";
    size.textContent = formatBytes(file.size);
    main.append(title, size);
    if (result?.status === "error") {
      const detail = document.createElement("span");
      detail.className = "file-error-detail";
      const message = [result.error?.code, result.error?.userMessage, result.error?.suggestion].filter(Boolean).join(" · ");
      detail.textContent = message || "转换失败";
      detail.title = result.error?.detail || message;
      main.append(detail);
    }
    const status = document.createElement("span");
    status.className = "file-status";
    if (result?.status === "ok") {
      status.dataset.state = "success";
      status.textContent = "已完成";
      status.title = result.output || "";
    } else if (result?.status === "error") {
      status.dataset.state = result.error?.code === "ERR_CANCELLED" ? "cancelled" : "error";
      status.textContent = result.error?.code === "ERR_CANCELLED" ? "已取消" : "失败";
      status.title = result.error?.userMessage || result.error?.detail || "转换失败";
    } else {
      status.dataset.state = state.isBusy ? "waiting" : "ready";
      status.textContent = state.isBusy ? "等待" : "待转换";
    }
    const side = document.createElement("span");
    side.className = "file-row-side";
    side.append(status);
    const actions = document.createElement("span");
    actions.className = "file-row-actions";
    if (result?.status === "ok" && result.output) {
      actions.append(
        createFileAction("试听", `试听 ${file.name} 的转换结果`, () => previewOutput(result.output, file.name)),
        createFileAction("定位", `在资源管理器中定位 ${file.name} 的转换结果`, () => locateOutput(result.output))
      );
    }
    if (result?.status === "error" && result.error?.code !== "ERR_CANCELLED") {
      actions.append(createFileAction("重试", `重新转换 ${file.name}`, () => startConversion([file], true)));
    }
    actions.append(createFileAction("移除", `从队列移除 ${file.name}`, () => removeQueuedFile(file), true));
    side.append(actions);
    item.append(main, side);
    elements.fileList.append(item);
  });
  syncControls();
}

function setProgress(percent, statusText) {
  const safePercent = Math.max(0, Math.min(100, Number(percent) || 0));
  elements.totalProgress.style.width = `${safePercent}%`;
  elements.totalProgressTrack.setAttribute("aria-valuenow", String(safePercent));
  elements.progressPercent.textContent = `${safePercent}%`;
  if (statusText) elements.progressStatus.textContent = statusText;
}

function estimatedRemainingText(percent) {
  const value = Number(percent) || 0;
  if (!state.taskStartedAt || value < 2 || value >= 100) return "";
  const elapsed = Date.now() - state.taskStartedAt;
  const remaining = elapsed * (100 - value) / value;
  if (!Number.isFinite(remaining) || remaining <= 0) return "";
  return ` · 预计剩余 ${formatDuration(remaining)}`;
}

function prepareProgress(preserveResults = false) {
  if (!preserveResults) state.fileResults.clear();
  state.lastOutputDir = "";
  resetPreview();
  elements.progressPanel.hidden = false;
  elements.resultSummary.hidden = true;
  elements.resultSummaryText.textContent = "";
  setProgress(0, preserveResults ? "正在重新提交失败项…" : "正在创建转换任务…");
  renderFiles();
}

function eventMatchesTask(event) {
  const taskId = String(event?.taskId || "");
  if (!taskId || !state.isBusy) return false;
  if (state.activeTaskId && state.activeTaskId !== taskId) return false;
  if (!state.activeTaskId) state.activeTaskId = taskId;
  return true;
}

function handleProgressEvent(event) {
  if (!eventMatchesTask(event)) return;
  const progress = event.progress || {};
  const phase = phaseLabels[progress.phase] || "处理中";
  const filename = progress.file ? `：${progress.file}` : "";
  setProgress(progress.percent, `${phase}${filename}${estimatedRemainingText(progress.percent)}`);
}

function handleFileDoneEvent(event) {
  if (!eventMatchesTask(event)) return;
  const result = event.result;
  if (!result) return;
  state.fileResults.set(pathKey(result.input || result.file), result);
  recordDiagnostic(result.status === "ok" ? "info" : "error", `文件处理${result.status === "ok" ? "完成" : "失败"}`, result.file || result.input);
  renderFiles();
  setProgress(result.percent, result.status === "ok" ? `已完成：${result.file}` : `处理失败：${result.file}`);
}

function handleCompleteEvent(event) {
  if (!eventMatchesTask(event)) return;
  const summary = event.summary || {};
  (summary.results || []).forEach((result) => { state.fileResults.set(pathKey(result.input || result.file), result); });
  state.isBusy = false;
  state.cancelRequested = false;
  state.activeTaskId = "";
  state.taskStartedAt = 0;
  state.lastOutputDir = summary.outputDir || elements.outputDirectory.value;
  const statusText = summary.cancelled
    ? `转换已取消：成功 ${summary.success || 0} 个，未完成或失败 ${summary.failed || 0} 个。`
    : `转换完成：成功 ${summary.success || 0} 个，失败 ${summary.failed || 0} 个。`;
  if (!summary.cancelled) setProgress(100, statusText);
  else elements.progressStatus.textContent = statusText;
  elements.statSuccess.textContent = String(summary.success || 0);
  elements.statFailed.textContent = String(summary.failed || 0);
  elements.statDuration.textContent = formatDuration(summary.durationMs);
  elements.resultSummaryText.textContent = `${statusText} 用时 ${formatDuration(summary.durationMs)}。`;
  elements.resultSummary.hidden = false;
  elements.openOutputButton.hidden = !state.lastOutputDir;
  appendHistory(summary);
  recordDiagnostic(summary.failed > 0 ? "warning" : "info", "转换任务结束", statusText);
  renderFiles();
}

function handleErrorEvent(event) {
  if (!eventMatchesTask(event)) return;
  state.isBusy = false;
  state.cancelRequested = false;
  state.activeTaskId = "";
  state.taskStartedAt = 0;
  const parts = [event.userMessage, event.suggestion].filter(Boolean);
  const message = parts.join(" ") || event.detail || "转换任务失败。";
  showConversionError(message);
  elements.progressStatus.textContent = "转换任务失败";
  recordDiagnostic("error", "转换任务失败", `${event.code || "ERR_UNKNOWN"} ${message} ${event.detail || ""}`.trim());
  renderFiles();
}

function subscribeToConversionEvents() {
  const runtime = window.runtime;
  if (!runtime?.EventsOn) return;
  runtime.EventsOn("conversion:progress", handleProgressEvent);
  runtime.EventsOn("conversion:file-done", handleFileDoneEvent);
  runtime.EventsOn("conversion:complete", handleCompleteEvent);
  runtime.EventsOn("conversion:error", handleErrorEvent);
}

function subscribeToFileDrop() {
  const runtime = window.runtime;
  if (!runtime?.OnFileDrop) return;
  runtime.OnFileDrop(async (_x, _y, paths) => {
    if (state.isBusy || state.isScanning || state.isAdvancedScanning) {
      showFileError("当前任务进行中，暂时不能拖入文件。");
      return;
    }
    const api = desktopApi();
    if (!api || !Array.isArray(paths) || paths.length === 0) return;
    showFileError();
    state.isScanning = true;
    elements.scanHelp.textContent = "正在处理拖入的文件和目录…";
    syncControls();
    try {
      const result = await api.ResolveDroppedPaths({ paths, recursive: elements.scanRecursive.checked });
      const stats = addFiles(result.files || []);
      const warning = (result.warnings || []).join(" ");
      elements.scanHelp.textContent = `拖入处理中找到 ${result.files?.length || 0} 个支持文件，新增 ${stats.added} 个。${warning}`;
      if (result.truncated) showFileError("拖入结果已达到 500 个队列上限。");
    } catch (error) {
      const message = errorText(error);
      showFileError(message);
      elements.scanHelp.textContent = "拖入的内容未能加入队列。";
      recordDiagnostic("error", "拖放处理失败", message);
    } finally {
      state.isScanning = false;
      syncControls();
    }
  }, true);
}

async function previewOutput(outputPath, fileName) {
  showConversionError();
  const api = desktopApi();
  if (!api) return;
  if (elements.previewAudio.dataset.output === outputPath && !elements.previewAudio.paused) {
    elements.previewAudio.pause();
    return;
  }
  try {
    const previewURL = await api.GetPreviewURL(outputPath);
    elements.previewAudio.src = previewURL;
    elements.previewAudio.dataset.output = outputPath;
    elements.previewFileName.textContent = fileName || outputPath;
    elements.previewPanel.hidden = false;
    await elements.previewAudio.play();
  } catch (error) {
    resetPreview();
    showConversionError(`无法试听输出文件：${errorText(error)}`);
  }
}

async function locateOutput(outputPath) {
  showConversionError();
  const api = desktopApi();
  if (!api) return;
  try {
    await api.OpenOutputFile(outputPath);
  } catch (error) {
    showConversionError(errorText(error));
  }
}

function readHistory() {
  try {
    const parsed = JSON.parse(localStorage.getItem(historyStorageKey) || "[]");
    return Array.isArray(parsed) ? parsed.slice(0, 50) : [];
  } catch {
    return [];
  }
}

function writeHistory(history) {
  try {
    localStorage.setItem(historyStorageKey, JSON.stringify(history.slice(0, 50)));
  } catch {
    recordDiagnostic("warning", "历史记录保存失败", "WebView 存储不可用");
  }
}

function appendHistory(summary) {
  if (!summary || Number(summary.total) <= 0) return;
  const history = readHistory();
  history.unshift({
    id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
    timestamp: new Date().toISOString(),
    total: Number(summary.total) || 0,
    success: Number(summary.success) || 0,
    failed: Number(summary.failed) || 0,
    durationMs: Number(summary.durationMs) || 0,
    outputDir: String(summary.outputDir || elements.outputDirectory.value || ""),
    outputFormat: String(summary.outputFormat || elements.formatSelect.value || "mp3"),
    mp3Quality: Number(summary.mp3Quality ?? elements.qualitySelect.value)
  });
  writeHistory(history);
  renderHistory();
}

function restoreHistoryItem(item) {
  if (!item || state.isBusy || state.isScanning || state.isAdvancedScanning) return;
  const format = String(item.outputFormat || "").toLowerCase();
  if (Array.from(elements.formatSelect.options).some((option) => option.value === format)) elements.formatSelect.value = format;
  const quality = String(Number(item.mp3Quality));
  if (Array.from(elements.qualitySelect.options).some((option) => option.value === quality)) elements.qualitySelect.value = quality;
  if (item.outputDir) elements.outputDirectory.value = String(item.outputDir);
  showOutputError();
  savePreferences();
  syncControls();
  elements.outputDirectory.focus();
}

async function deleteHistoryItem(item) {
  const api = desktopApi();
  if (!api || !(await api.ConfirmHistoryAction("delete"))) return;
  const history = readHistory();
  const key = String(item.id || item.timestamp || "");
  writeHistory(history.filter((entry) => String(entry.id || entry.timestamp || "") !== key));
  recordDiagnostic("info", "删除历史记录");
  renderHistory();
}

function renderHistory() {
  const history = readHistory();
  const visible = state.historyExpanded ? history : history.slice(0, 10);
  elements.historyList.replaceChildren();
  elements.historyEmpty.hidden = history.length > 0;
  elements.historyList.hidden = history.length === 0;
  elements.historyToggleButton.hidden = history.length <= 10;
  elements.historyToggleButton.textContent = state.historyExpanded ? "收起" : `显示全部（${history.length}）`;
  elements.clearHistoryButton.hidden = history.length === 0;
  visible.forEach((item) => {
    const row = document.createElement("li");
    row.className = "history-item";
    const summary = document.createElement("div");
    summary.className = "history-item-main";
    const time = document.createElement("strong");
    const date = new Date(item.timestamp);
    time.textContent = Number.isNaN(date.getTime()) ? "时间未知" : date.toLocaleString("zh-CN", { hour12: false });
    const meta = document.createElement("span");
    meta.textContent = `${String(item.outputFormat || "copy").toUpperCase()} · ${item.success || 0}/${item.total || 0} 成功 · ${formatDuration(item.durationMs)}`;
    const path = document.createElement("span");
    path.className = "history-item-path";
    path.textContent = item.outputDir || "未记录输出目录";
    path.title = item.outputDir || "";
    summary.append(time, meta, path);
    const actions = document.createElement("div");
    actions.className = "history-item-actions";
    actions.append(
      createFileAction("恢复设置", `恢复 ${time.textContent} 的转换设置`, () => restoreHistoryItem(item)),
      createFileAction("删除", `删除 ${time.textContent} 的历史记录`, () => deleteHistoryItem(item), true)
    );
    row.append(summary, actions);
    elements.historyList.append(row);
  });
}

function parseVersion(value) {
  const text = String(value || "").trim().replace(/^v/i, "");
  const [core, prerelease = ""] = text.split("-", 2);
  const parts = core.split(".").slice(0, 3).map((part) => Number.parseInt(part, 10));
  if (parts.some((part) => !Number.isFinite(part))) return null;
  while (parts.length < 3) parts.push(0);
  return { parts, prerelease };
}

function isNewerVersion(remoteValue, currentValue) {
  const remote = parseVersion(remoteValue);
  const current = parseVersion(currentValue);
  if (!remote || !current) return false;
  for (let index = 0; index < 3; index += 1) {
    if (remote.parts[index] !== current.parts[index]) return remote.parts[index] > current.parts[index];
  }
  return Boolean(current.prerelease) && !remote.prerelease;
}

function summarizeReleaseBody(body) {
  const lines = String(body || "")
    .split(/\r?\n/)
    .map((line) => line.replace(/^\s*[-*#>]+\s*/, "").replace(/\[([^\]]+)]\([^)]*\)/g, "$1").trim())
    .filter(Boolean)
    .slice(0, 3);
  const summary = lines.join("；");
  return summary.length > 220 ? `${summary.slice(0, 217)}…` : summary;
}

function showUpdateNotice(title, message, release = null) {
  elements.updateNoticeTitle.textContent = title;
  elements.updateStatus.textContent = message;
  elements.openUpdateButton.hidden = !release;
  elements.openReleasePageButton.hidden = !release;
  elements.ignoreUpdateButton.hidden = !release;
  elements.updateNotice.hidden = false;
  state.lastUpdateRelease = release;
}

function ignoredUpdateTag() {
  try {
    return localStorage.getItem(ignoredUpdateStorageKey) || "";
  } catch {
    return "";
  }
}

function presentRelease(release, manual) {
  if (!isNewerVersion(release?.tagName, state.currentVersion)) {
    if (manual) showUpdateNotice("当前已是最新版本", `当前版本 ${state.currentVersion || "未知"}，未发现可用更新。`);
    return;
  }
  if (!manual && ignoredUpdateTag() === release.tagName) return;
  state.lastUpdateURL = release.htmlUrl || "https://github.com/skxxxkx666/Kugo-Music-Converter/releases";
  const published = new Date(release.publishedAt || "");
  const dateText = Number.isNaN(published.getTime()) ? "" : `，发布于 ${published.toLocaleDateString("zh-CN")}`;
  const summary = summarizeReleaseBody(release.body);
  showUpdateNotice("发现新版本", `${release.tagName}${dateText}。${summary ? ` 更新摘要：${summary}` : ""}`, release);
}

async function checkForUpdates(manual = false) {
  if (state.isCheckingUpdate) return;
  const api = desktopApi();
  if (!api) return;
  state.isCheckingUpdate = true;
  syncControls();
  try {
    const release = await api.CheckForUpdates();
    try {
      localStorage.setItem(updateCheckStorageKey, JSON.stringify({ checkedAt: Date.now(), release }));
    } catch {
      recordDiagnostic("warning", "更新缓存写入失败");
    }
    presentRelease(release, manual);
    recordDiagnostic("info", "检查更新完成", release?.tagName || "无版本信息");
  } catch (error) {
    const message = errorText(error);
    if (manual) showUpdateNotice("检查更新失败", message);
    recordDiagnostic("warning", "检查更新失败", message);
  } finally {
    state.isCheckingUpdate = false;
    syncControls();
  }
}

function maybeCheckForUpdates() {
  try {
    const cached = JSON.parse(localStorage.getItem(updateCheckStorageKey) || "null");
    if (cached?.release) presentRelease(cached.release, false);
    if (Date.now() - Number(cached?.checkedAt || 0) < updateCheckInterval) return;
  } catch {
    recordDiagnostic("warning", "更新缓存无效");
  }
  checkForUpdates(false);
}

function isConvertibleScanFile(file) {
  return encryptedExtensions.has(String(file?.ext || "").toLowerCase());
}

function renderAdvancedFolders() {
  elements.advancedFolderList.replaceChildren();
  state.advancedFolders.forEach((path) => {
    const item = document.createElement("li");
    const label = document.createElement("span");
    label.textContent = path;
    label.title = path;
    const remove = createFileAction("移除", `移除扫描目录 ${path}`, () => {
      state.advancedFolders = state.advancedFolders.filter((entry) => pathKey(entry) !== pathKey(path));
      renderAdvancedFolders();
    }, true);
    item.append(label, remove);
    elements.advancedFolderList.append(item);
  });
  elements.clearAdvancedFoldersButton.hidden = state.advancedFolders.length === 0;
  elements.advancedScanStatus.textContent = state.advancedFolders.length
    ? `已选择 ${state.advancedFolders.length} 个目录。`
    : "先添加一个或多个本地目录。";
  syncControls();
}

function renderAdvancedResults(result) {
  state.advancedFiles = (result.folders || []).flatMap((folder) => folder.files || []);
  elements.advancedResult.hidden = state.advancedFiles.length === 0;
  elements.advancedResultList.replaceChildren();
  const convertible = state.advancedFiles.filter(isConvertibleScanFile).length;
  const warnings = (result.warnings || []).join(" ");
  elements.advancedResultSummary.textContent = `扫描到 ${result.totalFiles || state.advancedFiles.length} 个文件，共 ${formatBytes(result.totalSize)}；其中 ${convertible} 个可加入转换队列。${warnings ? ` ${warnings}` : ""}`;
  state.advancedFiles.slice(0, 500).forEach((file) => {
    const item = document.createElement("li");
    const main = document.createElement("span");
    main.className = "scan-file-main";
    const name = document.createElement("strong");
    name.textContent = file.name;
    name.title = file.fullPath;
    const path = document.createElement("span");
    path.textContent = file.fullPath;
    path.title = file.fullPath;
    main.append(name, path);
    const meta = document.createElement("span");
    meta.className = "scan-file-meta";
    meta.textContent = `${String(file.ext || "").toUpperCase() || "无扩展名"} · ${formatBytes(file.size)}`;
    item.append(main, meta);
    elements.advancedResultList.append(item);
  });
  if (state.advancedFiles.length > 500) {
    const remainder = document.createElement("li");
    remainder.className = "scan-result-remainder";
    remainder.textContent = `列表仅显示前 500 项；导出和复制仍包含全部 ${state.advancedFiles.length} 项。`;
    elements.advancedResultList.append(remainder);
  }
  syncControls();
}

function selectedAdvancedFilter() {
  if (elements.advancedFilter.value !== "custom") return elements.advancedFilter.value;
  return elements.customFilterInput.value.trim();
}

function csvCell(value) {
  return `"${String(value ?? "").replace(/"/g, '""')}"`;
}

function scanFilesCsv(files) {
  const rows = [["文件名", "扩展名", "大小(字节)", "修改时间", "完整路径"]];
  files.forEach((file) => rows.push([file.name, file.ext, file.size, file.modTime, file.fullPath]));
  return `\uFEFF${rows.map((row) => row.map(csvCell).join(",")).join("\r\n")}`;
}

function failedFilesCsv(entries) {
  const rows = [["文件名", "输入路径", "错误代码", "错误说明", "建议", "详细信息"]];
  entries.forEach(({ file, result }) => rows.push([
    file.name,
    file.path,
    result.error?.code || "",
    result.error?.userMessage || "",
    result.error?.suggestion || "",
    result.error?.detail || ""
  ]));
  return `\uFEFF${rows.map((row) => row.map(csvCell).join(",")).join("\r\n")}`;
}

async function saveText(defaultFilename, extension, content, statusElement = elements.diagnosticStatus) {
  const api = desktopApi();
  if (!api) return "";
  try {
    const path = await api.SaveTextFile({ defaultFilename, extension, content });
    statusElement.textContent = path ? `已保存：${path}` : "已取消保存。";
    if (path) recordDiagnostic("info", "导出文件", path);
    return path;
  } catch (error) {
    const message = errorText(error);
    statusElement.textContent = `导出失败：${message}`;
    recordDiagnostic("error", "导出失败", message);
    return "";
  }
}

async function copyText(text, successMessage) {
  if (!window.runtime?.ClipboardSetText) {
    showAdvancedScanError("桌面剪贴板不可用。");
    return;
  }
  try {
    await window.runtime.ClipboardSetText(text);
    elements.advancedScanStatus.textContent = successMessage;
  } catch (error) {
    showAdvancedScanError(errorText(error));
  }
}

function diagnosticContent() {
  const resultLines = state.files.map((file) => {
    const result = resultForFile(file);
    return `- ${file.name} | ${result?.status || "pending"} | ${result?.error?.code || ""}`;
  });
  const eventLines = state.diagnostics.map((entry) => `${entry.time} [${entry.level.toUpperCase()}] ${entry.message}${entry.detail ? ` | ${entry.detail}` : ""}`);
  return [
    "Kugo Music Converter 桌面诊断日志",
    `导出时间: ${new Date().toISOString()}`,
    `应用版本: ${state.currentVersion || "unknown"}`,
    `运行时就绪: ${state.runtimeReady}`,
    `运行时状态: ${elements.runtimeMessage.textContent}`,
    `输出格式: ${elements.formatSelect.value}`,
    `并发数: ${elements.concurrencySelect.value}`,
    `输出目录: ${elements.outputDirectory.value}`,
    `数据库状态: ${state.dbPath || "未检测到"}`,
    `队列文件数: ${state.files.length}`,
    "",
    "队列结果:",
    ...(resultLines.length ? resultLines : ["- 无"]),
    "",
    "本次运行事件:",
    ...(eventLines.length ? eventLines : ["- 无"])
  ].join("\r\n");
}

async function loadStartupState() {
  const api = desktopApi();
  if (!api) {
    elements.runtimeBadge.dataset.state = "error";
    elements.runtimeBadge.textContent = "桌面桥不可用";
    elements.runtimeMessage.textContent = "当前页面未运行在 Kugo 桌面应用中。";
    syncControls();
    return;
  }
  try {
    const startup = await api.GetStartupState();
    state.runtimeReady = Boolean(startup.runtimeReady);
    state.defaultConcurrency = Number(startup.defaultConcurrency) || 1;
    state.maxConcurrency = Number(startup.maxConcurrency) || state.defaultConcurrency;
    state.dbPath = startup.dbFound ? startup.dbPath || "" : "";
    state.currentVersion = startup.version || "dev";
    const preferences = readPreferences();
    elements.versionBadge.textContent = state.currentVersion;
    elements.outputDirectory.value = String(preferences.outputDir || startup.defaultOutputDir || "");
    elements.databasePath.value = state.dbPath;
    elements.runtimeMessage.textContent = startup.runtimeMessage || "运行时状态未知";
    elements.runtimeBadge.dataset.state = startup.runtimeReady ? "ready" : "warning";
    elements.runtimeBadge.textContent = startup.runtimeReady ? "运行时已就绪" : "开发运行时";
    const savedFormat = String(preferences.outputFormat || "");
    if (Array.from(elements.formatSelect.options).some((option) => option.value === savedFormat)) elements.formatSelect.value = savedFormat;
    const savedQuality = String(Number(preferences.mp3Quality));
    if (Array.from(elements.qualitySelect.options).some((option) => option.value === savedQuality)) elements.qualitySelect.value = savedQuality;
    populateConcurrencySelect(state.maxConcurrency, preferences.concurrency || state.defaultConcurrency);
    if (preferences.dbPath && pathKey(preferences.dbPath) !== pathKey(state.dbPath)) {
      try {
        const restoredPath = await api.RestoreDatabaseFile(String(preferences.dbPath));
        state.dbPath = restoredPath;
        elements.databasePath.value = restoredPath;
      } catch (error) {
        recordDiagnostic("warning", "已保存的数据库路径失效", errorText(error));
      }
    }
    const runtimeStillPreparing = !startup.runtimeReady && String(startup.runtimeMessage || "").includes("正在检查");
    if (runtimeStillPreparing) {
      syncControls();
      window.setTimeout(loadStartupState, 500);
      return;
    }
    recordDiagnostic("info", "桌面应用初始化完成", startup.runtimeMessage || "");
  } catch (error) {
    elements.runtimeBadge.dataset.state = "error";
    elements.runtimeBadge.textContent = "检查失败";
    elements.runtimeMessage.textContent = errorText(error);
    recordDiagnostic("error", "桌面应用初始化失败", errorText(error));
  }
  savePreferences();
  syncControls();
  maybeCheckForUpdates();
  refreshRuntimeCache();
}

async function startConversion(targetFiles = state.files, preserveResults = false) {
  showConversionError();
  showOutputError();
  const api = desktopApi();
  if (!api || state.isBusy) return;
  const files = Array.isArray(targetFiles) ? targetFiles.filter(Boolean) : [];
  if (files.length === 0) return showConversionError("没有可转换的文件。");
  if (!state.runtimeReady) return showConversionError("FFmpeg 运行时尚未就绪。");
  if (!elements.outputDirectory.value.trim()) return showConversionError("请先选择输出目录。");
  if (hasKGGFiles(files) && !state.dbPath) return showConversionError("所选文件包含 KGG，请先选择 KGMusicV3.db。");
  state.isBusy = true;
  state.cancelRequested = false;
  state.activeTaskId = "";
  state.taskStartedAt = Date.now();
  prepareProgress(preserveResults);
  savePreferences();
  syncControls();
  recordDiagnostic("info", preserveResults ? "开始重试转换" : "开始转换", `${files.length} 个文件`);
  try {
    const taskId = await api.StartConversion({
      paths: files.map((file) => file.path),
      outputDir: elements.outputDirectory.value.trim(),
      dbPath: state.dbPath,
      outputFormat: elements.formatSelect.value,
      mp3Quality: Number(elements.qualitySelect.value),
      concurrency: Number(elements.concurrencySelect.value) || state.defaultConcurrency
    });
    if (state.isBusy) {
      if (!state.activeTaskId) state.activeTaskId = taskId;
      elements.progressStatus.textContent = "转换任务已启动";
    }
  } catch (error) {
    state.isBusy = false;
    state.cancelRequested = false;
    state.activeTaskId = "";
    state.taskStartedAt = 0;
    const message = errorText(error);
    showConversionError(message);
    elements.progressStatus.textContent = "无法启动转换任务";
    recordDiagnostic("error", "无法启动转换任务", message);
    renderFiles();
  }
}

elements.scanFolderButton.addEventListener("click", async () => {
  showScanError();
  const api = desktopApi();
  if (!api) return showScanError("桌面桥不可用，无法扫描文件夹。");
  state.isScanning = true;
  elements.scanHelp.textContent = "请选择要扫描的文件夹。";
  syncControls();
  try {
    const directory = await api.SelectScanDirectory();
    if (!directory) {
      elements.scanHelp.textContent = "未选择文件夹。";
      return;
    }
    elements.scanHelp.textContent = `正在扫描：${directory}`;
    const result = await api.ScanAudioDirectory({ path: directory, recursive: elements.scanRecursive.checked });
    const stats = addFiles(result.files);
    const warning = result.warning ? ` ${result.warning}` : "";
    elements.scanHelp.textContent = `扫描到 ${result.files?.length || 0} 个支持文件，已新增 ${stats.added} 个。${warning}`;
  } catch (error) {
    elements.scanHelp.textContent = "文件夹扫描未完成。";
    showScanError(errorText(error));
  } finally {
    state.isScanning = false;
    syncControls();
  }
});

elements.pickFilesButton.addEventListener("click", async () => {
  showFileError();
  const api = desktopApi();
  if (!api) return showFileError("桌面桥不可用，无法打开文件选择器。");
  elements.pickFilesButton.disabled = true;
  try {
    const selected = await api.SelectAudioFiles();
    if (Array.isArray(selected) && selected.length > 0) addFiles(selected);
  } catch (error) {
    showFileError(errorText(error));
  } finally {
    syncControls();
  }
});

function triggerPickFiles() {
  if (!elements.pickFilesButton.disabled) elements.pickFilesButton.click();
}

elements.emptyQueue.addEventListener("click", triggerPickFiles);
elements.emptyQueue.addEventListener("keydown", (event) => {
  if (event.key === "Enter" || event.key === " ") {
    event.preventDefault();
    triggerPickFiles();
  }
});

elements.clearFilesButton.addEventListener("click", () => {
  state.files = [];
  state.fileResults.clear();
  state.selectedPath = "";
  closeContextMenu();
  elements.progressPanel.hidden = true;
  resetPreview();
  showFileError();
  showConversionError();
  recordDiagnostic("info", "清空转换队列");
  renderFiles();
  elements.pickFilesButton.focus();
});

elements.pickOutputButton.addEventListener("click", async () => {
  showOutputError();
  const api = desktopApi();
  if (!api) return showOutputError("桌面桥不可用，无法打开目录选择器。");
  elements.pickOutputButton.disabled = true;
  try {
    const directory = await api.SelectOutputDirectory();
    if (directory) {
      elements.outputDirectory.value = directory;
      savePreferences();
    }
  } catch (error) {
    showOutputError(errorText(error));
  } finally {
    syncControls();
  }
});

elements.openConfiguredOutputButton.addEventListener("click", async () => {
  showOutputError();
  const api = desktopApi();
  const directory = elements.outputDirectory.value.trim();
  if (!api || !directory) return;
  try {
    await api.OpenOutputDirectory(directory);
  } catch (error) {
    showOutputError(errorText(error));
  }
});

elements.pickDatabaseButton.addEventListener("click", async () => {
  showDatabaseError();
  const api = desktopApi();
  if (!api) return showDatabaseError("桌面桥不可用，无法打开数据库选择器。");
  elements.pickDatabaseButton.disabled = true;
  try {
    const path = await api.SelectDatabaseFile();
    if (path) {
      state.dbPath = path;
      elements.databasePath.value = path;
      savePreferences();
      recordDiagnostic("info", "选择 KGG 数据库", path);
    }
  } catch (error) {
    showDatabaseError(errorText(error));
  } finally {
    syncControls();
  }
});

elements.redetectDatabaseButton.addEventListener("click", async () => {
  showDatabaseError();
  const api = desktopApi();
  if (!api) return showDatabaseError("桌面桥不可用，无法重新检测数据库。");
  elements.redetectDatabaseButton.disabled = true;
  try {
    const status = await api.RedetectDatabase();
    state.dbPath = status.found ? status.path || "" : "";
    elements.databasePath.value = state.dbPath;
    if (!status.found) showDatabaseError("未自动检测到 KGMusicV3.db，可使用“选择”手动指定。");
    else recordDiagnostic("info", "重新检测到 KGG 数据库", `${status.source}: ${status.path}`);
    savePreferences();
  } catch (error) {
    showDatabaseError(errorText(error));
  } finally {
    syncControls();
  }
});

[elements.formatSelect, elements.qualitySelect, elements.concurrencySelect].forEach((control) => {
  control.addEventListener("change", () => {
    savePreferences();
    syncControls();
  });
});

elements.convertButton.addEventListener("click", () => startConversion());
elements.cancelButton.addEventListener("click", async () => {
  if (!state.activeTaskId || state.cancelRequested) return;
  const api = desktopApi();
  if (!api) return;
  try {
    const confirmed = await api.ConfirmConversionCancellation();
    if (!confirmed) return;
    state.cancelRequested = true;
    syncControls();
    elements.progressStatus.textContent = "正在取消转换…";
    const accepted = await api.CancelConversion(state.activeTaskId);
    if (!accepted) {
      state.cancelRequested = false;
      showConversionError("任务已结束或无法取消。");
      syncControls();
    } else {
      recordDiagnostic("warning", "请求取消转换任务", state.activeTaskId);
    }
  } catch (error) {
    state.cancelRequested = false;
    showConversionError(errorText(error));
    syncControls();
  }
});

elements.retryFailedButton.addEventListener("click", () => startConversion(failedEntries().map(({ file }) => file), true));
elements.exportFailedButton.addEventListener("click", () => {
  const entries = failedEntries();
  if (entries.length > 0) saveText("Kugo-转换失败列表.csv", ".csv", failedFilesCsv(entries), elements.progressStatus);
});
elements.openOutputButton.addEventListener("click", async () => {
  showConversionError();
  const api = desktopApi();
  if (!api || !state.lastOutputDir) return;
  try {
    await api.OpenOutputDirectory(state.lastOutputDir);
  } catch (error) {
    showConversionError(errorText(error));
  }
});

elements.historyToggleButton.addEventListener("click", () => {
  state.historyExpanded = !state.historyExpanded;
  renderHistory();
});
elements.clearHistoryButton.addEventListener("click", async () => {
  const api = desktopApi();
  if (!api || !(await api.ConfirmHistoryAction("clear"))) return;
  writeHistory([]);
  state.historyExpanded = false;
  recordDiagnostic("info", "清空全部历史记录");
  renderHistory();
  syncControls();
});

elements.checkUpdateButton.addEventListener("click", () => checkForUpdates(true));
elements.openUpdateButton.addEventListener("click", async () => {
  const api = desktopApi();
  if (!api || !state.lastUpdateRelease?.tagName || state.isInstallingUpdate) return;
  state.isInstallingUpdate = true;
  syncControls();
  try {
    const result = await api.DownloadAndInstallUpdate(state.lastUpdateRelease.tagName);
    if (!result?.started) showUpdateNotice("更新已取消", result?.message || "未启动安装器", state.lastUpdateRelease);
  } catch (error) {
    showUpdateNotice("自动更新未完成", `${errorText(error)}。可使用“查看 Releases”手动更新。`, state.lastUpdateRelease);
    recordDiagnostic("error", "自动更新未完成", errorText(error));
  } finally {
    state.isInstallingUpdate = false;
    syncControls();
  }
});
elements.openReleasePageButton.addEventListener("click", async () => {
  const api = desktopApi();
  if (!api || !state.lastUpdateURL) return;
  try {
    await api.OpenReleasePage(state.lastUpdateURL);
  } catch (error) {
    showUpdateNotice("无法打开更新页面", errorText(error), state.lastUpdateRelease);
  }
});
elements.ignoreUpdateButton.addEventListener("click", () => {
  if (state.lastUpdateRelease?.tagName) {
    try {
      localStorage.setItem(ignoredUpdateStorageKey, state.lastUpdateRelease.tagName);
    } catch {
      recordDiagnostic("warning", "无法保存忽略更新设置");
    }
  }
  elements.updateNotice.hidden = true;
});
elements.dismissUpdateButton.addEventListener("click", () => { elements.updateNotice.hidden = true; });

elements.addAdvancedFolderButton.addEventListener("click", async () => {
  const api = desktopApi();
  if (!api) return;
  showAdvancedScanError();
  try {
    const directory = await api.SelectScanDirectory();
    if (!directory) return;
    if (!state.advancedFolders.some((entry) => pathKey(entry) === pathKey(directory))) state.advancedFolders.push(directory);
    renderAdvancedFolders();
  } catch (error) {
    showAdvancedScanError(errorText(error));
  }
});

elements.clearAdvancedFoldersButton.addEventListener("click", () => {
  state.advancedFolders = [];
  state.advancedFiles = [];
  elements.advancedResult.hidden = true;
  renderAdvancedFolders();
});

elements.advancedFilter.addEventListener("change", () => {
  elements.customFilterField.hidden = elements.advancedFilter.value !== "custom";
  if (!elements.customFilterField.hidden) elements.customFilterInput.focus();
});

elements.runAdvancedScanButton.addEventListener("click", async () => {
  const api = desktopApi();
  if (!api || state.advancedFolders.length === 0) return;
  showAdvancedScanError();
  const filter = selectedAdvancedFilter();
  if (elements.advancedFilter.value === "custom" && !filter) {
    showAdvancedScanError("请输入至少一个扩展名，例如 .mp3,.flac。");
    elements.customFilterInput.focus();
    return;
  }
  state.isAdvancedScanning = true;
  elements.advancedScanStatus.textContent = "正在扫描所选目录…";
  syncControls();
  try {
    const result = await api.ScanDirectories({
      paths: state.advancedFolders,
      recursive: elements.advancedRecursive.checked,
      filter
    });
    renderAdvancedResults(result);
    elements.advancedScanStatus.textContent = `扫描完成：${result.totalFiles || 0} 个文件。`;
    recordDiagnostic("info", "高级目录扫描完成", `${result.totalFiles || 0} 个文件`);
  } catch (error) {
    const message = errorText(error);
    showAdvancedScanError(message);
    elements.advancedScanStatus.textContent = "扫描未完成。";
    recordDiagnostic("error", "高级目录扫描失败", message);
  } finally {
    state.isAdvancedScanning = false;
    syncControls();
  }
});

elements.copyAdvancedNamesButton.addEventListener("click", () => {
  copyText(state.advancedFiles.map((file) => file.name).join("\r\n"), `已复制 ${state.advancedFiles.length} 个文件名。`);
});
elements.copyAdvancedPathsButton.addEventListener("click", () => {
  copyText(state.advancedFiles.map((file) => file.fullPath).join("\r\n"), `已复制 ${state.advancedFiles.length} 条完整路径。`);
});
elements.exportAdvancedCsvButton.addEventListener("click", () => {
  saveText("Kugo-目录扫描结果.csv", ".csv", scanFilesCsv(state.advancedFiles), elements.advancedScanStatus);
});
elements.addAdvancedQueueButton.addEventListener("click", () => {
  const files = state.advancedFiles.filter(isConvertibleScanFile).map((file) => ({
    path: file.fullPath,
    name: file.name,
    size: file.size
  }));
  const stats = addFiles(files);
  elements.advancedScanStatus.textContent = `已向转换队列新增 ${stats.added} 个文件；重复 ${stats.duplicate} 个，超限 ${stats.blocked} 个。`;
});
elements.exportDiagnosticButton.addEventListener("click", () => {
  const stamp = new Date().toISOString().replace(/[:.]/g, "-");
  saveText(`Kugo-桌面诊断-${stamp}.log`, ".log", diagnosticContent());
});

async function refreshRuntimeCache() {
  const api = desktopApi();
  if (!api || state.isManagingCache) return;
  state.isManagingCache = true;
  syncControls();
  try {
    const info = await api.GetRuntimeCacheInfo();
    state.runtimeCacheInfo = info;
    const reclaimable = Number(info?.reclaimableBytes || 0);
    const retained = Number(info?.retainedBytes || 0);
    const items = Number(info?.reclaimableItems || 0);
    elements.runtimeCacheStatus.textContent = items > 0
      ? `可清理 ${formatBytes(reclaimable)}（${items} 项）；当前运行时占用 ${formatBytes(retained)}，将保留。`
      : `没有可清理的旧缓存；当前运行时占用 ${formatBytes(retained)}。`;
  } catch (error) {
    state.runtimeCacheInfo = null;
    elements.runtimeCacheStatus.textContent = `缓存统计失败：${errorText(error)}`;
    recordDiagnostic("warning", "运行时缓存统计失败", errorText(error));
  } finally {
    state.isManagingCache = false;
    syncControls();
  }
}

elements.refreshRuntimeCacheButton.addEventListener("click", refreshRuntimeCache);
elements.clearRuntimeCacheButton.addEventListener("click", async () => {
  const api = desktopApi();
  if (!api || state.isManagingCache) return;
  state.isManagingCache = true;
  syncControls();
  try {
    const result = await api.ClearRuntimeCache();
    state.runtimeCacheInfo = result?.remainingInfo || null;
    if (result?.cancelled) {
      elements.runtimeCacheStatus.textContent = "已取消缓存清理。";
    } else if (Number(result?.removedItems || 0) > 0) {
      const warning = result?.warning ? ` ${result.warning}` : "";
      elements.runtimeCacheStatus.textContent = `已清理 ${formatBytes(Number(result.freedBytes || 0))}（${result.removedItems} 项）。${warning}`;
      recordDiagnostic("info", "运行时缓存清理完成", `${result.removedItems} 项，${formatBytes(Number(result.freedBytes || 0))}`);
    } else {
      elements.runtimeCacheStatus.textContent = "没有需要清理的旧缓存。";
    }
  } catch (error) {
    elements.runtimeCacheStatus.textContent = `缓存清理失败：${errorText(error)}`;
    recordDiagnostic("error", "运行时缓存清理失败", errorText(error));
  } finally {
    state.isManagingCache = false;
    syncControls();
  }
});

/* ───── 查找本机音乐 ───── */

function showFindMusicError(message = "") {
  setInlineError(elements.findMusicError, null, message);
}

function findMusicStats() {
  let total = 0;
  let checked = 0;
  state.findMusicGroups.forEach((group) => {
    total += group.files.length;
    group.files.forEach((file) => {
      if (state.findMusicChecked.has(file.path)) checked += 1;
    });
  });
  return { total, checked, sources: state.findMusicGroups.length };
}

function syncFindMusic() {
  const stats = findMusicStats();
  const hasResults = stats.total > 0;
  elements.findMusicResult.hidden = !hasResults;
  elements.findMusicClearButton.hidden = !hasResults && state.findMusicGroups.length === 0;
  elements.findMusicStatus.hidden = hasResults;
  if (hasResults) {
    const warning = state.findMusicWarning ? ` · ${state.findMusicWarning}` : "";
    elements.findMusicSummary.textContent = `检测到 ${stats.total} 个文件，来自 ${stats.sources} 个音乐软件 · 已选 ${stats.checked} 个${warning}`;
    elements.findMusicSelectAll.checked = stats.checked === stats.total;
    elements.findMusicSelectAll.indeterminate = stats.checked > 0 && stats.checked < stats.total;
  }
  elements.findMusicAddButton.disabled = stats.checked === 0 || state.isBusy || state.isScanning || state.isAdvancedScanning;
}

function renderFindMusicResults() {
  const list = elements.findMusicList;
  list.replaceChildren();
  state.findMusicGroups.forEach((group) => {
    const header = document.createElement("li");
    header.className = "find-music-group-header";
    let icon;
    if (group.iconImg) {
      icon = document.createElement("img");
      icon.className = "brand-icon";
      icon.src = group.iconImg;
      icon.alt = "";
      icon.setAttribute("aria-hidden", "true");
    } else {
      icon = document.createElementNS("http://www.w3.org/2000/svg", "svg");
      icon.setAttribute("class", "brand-icon");
      icon.setAttribute("aria-hidden", "true");
      const use = document.createElementNS("http://www.w3.org/2000/svg", "use");
      use.setAttribute("href", `#${group.icon}`);
      icon.append(use);
    }
    const name = document.createElement("span");
    name.textContent = group.name;
    const groupToggleLabel = document.createElement("label");
    groupToggleLabel.className = "group-toggle-label";
    const groupToggle = document.createElement("input");
    groupToggle.type = "checkbox";
    const groupPaths = group.files.map((file) => file.path);
    const groupChecked = groupPaths.filter((path) => state.findMusicChecked.has(path)).length;
    groupToggle.checked = groupPaths.length > 0 && groupChecked === groupPaths.length;
    groupToggle.indeterminate = groupChecked > 0 && groupChecked < groupPaths.length;
    groupToggle.setAttribute("aria-label", `选中 ${group.name} 的全部文件`);
    groupToggle.addEventListener("click", (event) => event.stopPropagation());
    groupToggle.addEventListener("change", () => {
      if (groupToggle.checked) groupPaths.forEach((path) => state.findMusicChecked.add(path));
      else groupPaths.forEach((path) => state.findMusicChecked.delete(path));
      renderFindMusicResults();
    });
    groupToggleLabel.append(groupToggle, document.createTextNode("全选"));
    const count = document.createElement("span");
    count.className = "count";
    count.textContent = `${group.files.length} 个文件`;
    header.append(icon, name, groupToggleLabel, count);
    list.append(header);
    group.files.forEach((file) => {
      const item = document.createElement("li");
      item.className = "find-music-item";
      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.checked = state.findMusicChecked.has(file.path);
      checkbox.setAttribute("aria-label", `选择 ${file.name}`);
      checkbox.addEventListener("change", () => {
        if (checkbox.checked) state.findMusicChecked.add(file.path);
        else state.findMusicChecked.delete(file.path);
        syncFindMusic();
      });
      item.addEventListener("click", (event) => {
        if (event.target === checkbox) return;
        checkbox.checked = !checkbox.checked;
        if (checkbox.checked) state.findMusicChecked.add(file.path);
        else state.findMusicChecked.delete(file.path);
        syncFindMusic();
      });
      const info = document.createElement("span");
      info.className = "file-info";
      const nameSpan = document.createElement("span");
      nameSpan.className = "file-name";
      nameSpan.textContent = file.name;
      nameSpan.title = file.path;
      const pathSpan = document.createElement("span");
      pathSpan.className = "file-path";
      pathSpan.textContent = file.path;
      info.append(nameSpan, pathSpan);
      const meta = document.createElement("span");
      meta.className = "file-meta";
      meta.textContent = formatBytes(file.size);
      item.append(checkbox, createExtBadge(file.name), info, meta);
      list.append(item);
    });
  });
  syncFindMusic();
}

async function runFindMusic() {
  if (state.isFindingMusic) return;
  showFindMusicError();
  const api = desktopApi();
  if (!api) return showFindMusicError("桌面桥不可用，无法查找本机音乐。");
  elements.findMusicPanel.hidden = false;
  elements.findMusicResult.hidden = true;
  elements.findMusicStatus.hidden = false;
  elements.findMusicStatus.textContent = "正在检测常见音乐软件的默认下载目录…";
  state.isFindingMusic = true;
  state.findMusicWarning = "";
  syncControls();
  recordDiagnostic("info", "开始查找本机音乐");
  try {
    const result = await api.FindLocalMusic();
    state.findMusicGroups = Array.isArray(result?.groups) ? result.groups : [];
    const warnings = Array.isArray(result?.warnings) ? result.warnings.filter(Boolean) : [];
    state.findMusicWarning = warnings.join(" ");
    state.findMusicChecked = new Set();
    state.findMusicGroups.forEach((group) => group.files.forEach((file) => state.findMusicChecked.add(file.path)));
    renderFindMusicResults();
    const stats = findMusicStats();
    if (stats.total === 0) {
      elements.findMusicStatus.hidden = false;
      elements.findMusicStatus.textContent = state.findMusicWarning || "未在常见音乐软件的下载目录中发现支持的加密音乐文件。";
    }
    recordDiagnostic("info", "本机音乐检测完成", `${stats.sources} 个来源，${stats.total} 个文件`);
  } catch (error) {
    const message = errorText(error);
    state.findMusicGroups = [];
    state.findMusicChecked = new Set();
    elements.findMusicStatus.textContent = "本机音乐检测未完成。";
    showFindMusicError(message);
    recordDiagnostic("error", "本机音乐检测失败", message);
  } finally {
    state.isFindingMusic = false;
    syncControls();
  }
}

elements.findMusicButton.addEventListener("click", () => {
  if (!elements.findMusicPanel.hidden && state.findMusicGroups.length > 0) {
    elements.findMusicPanel.hidden = true;
    return;
  }
  runFindMusic();
});

elements.findMusicSelectAll.addEventListener("change", () => {
  const checked = elements.findMusicSelectAll.checked;
  state.findMusicChecked = new Set();
  if (checked) {
    state.findMusicGroups.forEach((group) => group.files.forEach((file) => state.findMusicChecked.add(file.path)));
  }
  renderFindMusicResults();
});

elements.findMusicAddButton.addEventListener("click", () => {
  const selected = [];
  state.findMusicGroups.forEach((group) => {
    group.files.forEach((file) => {
      if (state.findMusicChecked.has(file.path)) selected.push({ path: file.path, name: file.name, size: file.size });
    });
  });
  if (selected.length === 0) return;
  const stats = addFiles(selected);
  elements.findMusicStatus.hidden = false;
  elements.findMusicStatus.textContent = `已向转换队列新增 ${stats.added} 个文件；重复 ${stats.duplicate} 个，超限 ${stats.blocked} 个。`;
  recordDiagnostic("info", "查找结果加入队列", `新增 ${stats.added} 个`);
});

elements.findMusicCollapseButton.addEventListener("click", () => {
  const collapsed = elements.findMusicPanel.classList.toggle("collapsed");
  elements.findMusicCollapseButton.setAttribute("aria-expanded", collapsed ? "false" : "true");
  elements.findMusicCollapseButton.setAttribute("aria-label", collapsed ? "展开查找结果" : "折叠查找结果");
});

elements.findMusicClearButton.addEventListener("click", () => {
  state.findMusicGroups = [];
  state.findMusicChecked = new Set();
  state.findMusicWarning = "";
  elements.findMusicList.replaceChildren();
  elements.findMusicResult.hidden = true;
  elements.findMusicStatus.hidden = false;
  elements.findMusicStatus.textContent = "点击“查找本机音乐”开始检测。";
  elements.findMusicClearButton.hidden = true;
  showFindMusicError();
});

/* ───── 首次打开开源声明 ───── */
function initFirstRunDisclaimer() {
  let seen = "";
  try {
    seen = localStorage.getItem(disclaimerStorageKey) || "";
  } catch {
    seen = "";
  }
  if (seen === "1") return;
  elements.firstRunOverlay.hidden = false;
  elements.firstRunConfirm.focus();
}

elements.firstRunConfirm.addEventListener("click", () => {
  try {
    localStorage.setItem(disclaimerStorageKey, "1");
  } catch {
    /* 存储不可用时下次仍会提示 */
  }
  elements.firstRunOverlay.hidden = true;
  recordDiagnostic("info", "已确认开源声明");
});

/* ───── GitHub 项目入口 ───── */
elements.githubLinkButton.addEventListener("click", () => {
  const url = "https://github.com/skxxxkx666/Kugo-Music-Converter";
  if (window.runtime?.BrowserOpenURL) {
    window.runtime.BrowserOpenURL(url);
  } else {
    window.open(url, "_blank", "noopener");
  }
});

/* ───── 全局键盘快捷键与菜单关闭 ───── */
function isEditableTarget(target) {
  if (!target || target === document.body) return false;
  const tag = target.tagName;
  return tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA" || target.isContentEditable;
}

document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    if (!elements.contextMenu.hidden) {
      event.preventDefault();
      closeContextMenu();
      return;
    }
    if (state.isBusy && !state.cancelRequested && !elements.cancelButton.hidden) {
      event.preventDefault();
      elements.cancelButton.click();
    }
    return;
  }
  if (isEditableTarget(event.target)) return;
  if ((event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && event.key.toLowerCase() === "o") {
    event.preventDefault();
    triggerPickFiles();
    return;
  }
  if (event.ctrlKey || event.metaKey || event.altKey) return;
  if (event.key === "Delete" || event.key === "Backspace") {
    const file = selectedQueueFile();
    if (file && !state.isBusy && !state.isScanning && !state.isAdvancedScanning) {
      event.preventDefault();
      removeQueuedFile(file);
    }
    return;
  }
  if (event.key === "Enter" && (event.target === document.body || event.target.classList?.contains("file-row"))) {
    if (!elements.convertButton.hidden && !elements.convertButton.disabled) {
      event.preventDefault();
      elements.convertButton.click();
    }
  }
});

document.addEventListener("click", (event) => {
  if (!elements.contextMenu.hidden && !elements.contextMenu.contains(event.target)) closeContextMenu();
});
document.addEventListener("contextmenu", (event) => {
  if (!elements.contextMenu.hidden && !elements.contextMenu.contains(event.target) && !event.target.closest?.(".file-row")) closeContextMenu();
});
window.addEventListener("resize", closeContextMenu);
window.addEventListener("blur", closeContextMenu);
elements.fileList.addEventListener("scroll", closeContextMenu);

subscribeToConversionEvents();
subscribeToFileDrop();
initTheme();
initFirstRunDisclaimer();
elements.themeToggleButton.addEventListener("click", () => {
  const next = effectiveThemeIsDark() ? "light" : "dark";
  document.documentElement.dataset.theme = next;
  try {
    localStorage.setItem(themeStorageKey, next);
  } catch {
    recordDiagnostic("warning", "主题偏好保存失败", "WebView 存储不可用");
  }
  syncThemeToggle();
});
populateConcurrencySelect(1, 1);
renderFiles();
renderHistory();
renderAdvancedFolders();
renderDiagnostics();
syncFormatSettings();
loadStartupState();
