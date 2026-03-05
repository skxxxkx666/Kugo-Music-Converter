export function createConfigController(options) {
  const {
    state,
    elements,
    storage,
    helpers,
    callbacks
  } = options;

  const {
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
  } = elements;

  const {
    storageKeys,
    themeKey,
    configCollapsedKey = "kgg-converter-config-collapsed"
  } = storage;

  const {
    fetchJson,
    appendLog,
    setHintStatus,
    setButtonContent,
    refreshIcons,
    toastSuccess,
    toastError,
    toastInfo
  } = helpers;

  const {
    requiresDb,
    isDbReady,
    onStateChanged
  } = callbacks;

  let validateDbTimer = null;
  let useSystemThemeSync = true;
  let syncingConfigDetails = false;
  const pickerTimeoutMs = 5 * 60 * 1000;

  function emitStateChanged() {
    if (typeof onStateChanged === "function") onStateChanged();
  }

  function savePreferences() {
    localStorage.setItem(storageKeys.outputDir, outputDirInput.value.trim());
    localStorage.setItem(storageKeys.dbPath, dbPathInput.value.trim());
    localStorage.setItem(storageKeys.outputFormat, outputFormatSelect.value);
    localStorage.setItem(storageKeys.mp3Quality, mp3QualitySelect.value);
    localStorage.setItem(storageKeys.concurrency, concurrencySelect.value);
  }

  function loadPreferences() {
    const outputDir = localStorage.getItem(storageKeys.outputDir);
    const dbPath = localStorage.getItem(storageKeys.dbPath);
    const outputFormat = localStorage.getItem(storageKeys.outputFormat);
    const mp3Quality = localStorage.getItem(storageKeys.mp3Quality);
    const concurrency = localStorage.getItem(storageKeys.concurrency);

    if (outputDir) outputDirInput.value = outputDir;
    if (dbPath) dbPathInput.value = dbPath;
    if (outputFormat) outputFormatSelect.value = outputFormat;
    if (mp3Quality) mp3QualitySelect.value = mp3Quality;
    if (concurrency) concurrencySelect.value = concurrency;
  }

  function applyTheme(theme, persist = true, useTransition = true) {
    const normalized = theme === "dark" ? "dark" : "light";
    const reduceMotion =
      typeof window !== "undefined" &&
      window.matchMedia &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (useTransition && !reduceMotion) {
      document.documentElement.classList.add("theme-transitioning");
      window.setTimeout(() => {
        document.documentElement.classList.remove("theme-transitioning");
      }, 420);
    }
    const isDark = normalized === "dark";
    document.documentElement.classList.toggle("dark", isDark);
    // 保留 data-theme 兼容已有逻辑与旧样式
    document.documentElement.setAttribute("data-theme", normalized);
    // 主题图标通过 CSS dark:hidden / hidden dark:block 自动切换，无需 JS 替换
    if (themeToggleBtn) {
      themeToggleBtn.setAttribute("aria-label", isDark ? "切换到浅色主题" : "切换到深色主题");
    }
    if (persist) localStorage.setItem(themeKey, normalized);
  }

  function initTheme() {
    const saved = localStorage.getItem(themeKey);
    useSystemThemeSync = !saved;

    if (saved === "light" || saved === "dark") {
      applyTheme(saved, false, false);
    } else {
      const prefersDark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
      applyTheme(prefersDark ? "dark" : "light", false, false);
    }

    if (window.matchMedia) {
      const media = window.matchMedia("(prefers-color-scheme: dark)");
      media.addEventListener("change", (event) => {
        if (useSystemThemeSync) applyTheme(event.matches ? "dark" : "light", false);
      });
    }
  }

  function toggleTheme() {
    useSystemThemeSync = false;
    const isDark = document.documentElement.classList.contains("dark");
    applyTheme(isDark ? "light" : "dark");
  }

  function updateMp3QualityVisibility() {
    mp3QualityWrap.classList.toggle("hidden", outputFormatSelect.value !== "mp3");
  }

  function buildConfigSummaryText() {
    const output = outputDirInput.value.trim() || "未设置";
    const format = String(outputFormatSelect.value || "copy").toUpperCase();
    const concurrency = String(concurrencySelect.value || "-");
    const dbText = isDbReady() ? "已就绪" : "未就绪";
    return `输出: ${output} | 格式: ${format} | 并发: ${concurrency} | DB: ${dbText}`;
  }

  function isConfigReadyForCollapse() {
    const runtimeReady = state.missingTools.length === 0;
    const outputReady = Boolean(outputDirInput.value.trim());
    const dbReady = requiresDb() ? isDbReady() : true;
    return runtimeReady && outputReady && dbReady;
  }

  function syncConfigCollapsedUi(persist = true) {
    if (configSummary) {
      configSummary.textContent = buildConfigSummaryText();
      configSummary.classList.toggle("hidden", !state.configCollapsed);
    }
    if (configToggleBtn) {
      setButtonContent(
        configToggleBtn,
        state.configCollapsed ? "展开配置" : "折叠配置",
        state.configCollapsed ? "panel-top-open" : "panel-top-close"
      );
      configToggleBtn.setAttribute("aria-label", state.configCollapsed ? "展开环境配置" : "折叠环境配置");
      refreshIcons();
    }
    if (persist) {
      localStorage.setItem(configCollapsedKey, state.configCollapsed ? "1" : "0");
    }
  }

  function setConfigCollapsed(collapsed, persist = true) {
    state.configCollapsed = Boolean(collapsed);
    if (configCard) {
      if (typeof configCard.open === "boolean") {
        syncingConfigDetails = true;
        configCard.open = !state.configCollapsed;
        queueMicrotask(() => {
          syncingConfigDetails = false;
        });
      } else {
        configCard.classList.toggle("collapsed", state.configCollapsed);
      }
    }
    syncConfigCollapsedUi(persist);
  }

  function handleConfigDetailsToggle() {
    if (!configCard || typeof configCard.open !== "boolean") return;
    state.configCollapsed = !configCard.open;
    syncConfigCollapsedUi(!syncingConfigDetails);
  }

  function updateConfigAutoCollapse() {
    const preferred = localStorage.getItem(configCollapsedKey);
    if (preferred === "0") {
      setConfigCollapsed(false, false);
      return;
    }
    if (preferred === "1") {
      setConfigCollapsed(true, false);
      return;
    }
    if (isConfigReadyForCollapse() && !state.isBusy) {
      setConfigCollapsed(true, false);
    } else {
      setConfigCollapsed(false, false);
    }
  }

  if (configCard && typeof configCard.addEventListener === "function" && typeof configCard.open === "boolean") {
    configCard.addEventListener("toggle", handleConfigDetailsToggle);
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

    emitStateChanged();
    updateConfigAutoCollapse();
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
      emitStateChanged();
      updateConfigAutoCollapse();
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

    emitStateChanged();
    updateConfigAutoCollapse();
  }

  function scheduleDbValidation() {
    clearTimeout(validateDbTimer);
    validateDbTimer = setTimeout(() => {
      validateDbPath(dbPathInput.value);
    }, 250);
  }

  async function fetchJsonWithTimeout(url, options = {}, timeoutMs = pickerTimeoutMs) {
    const controller = new AbortController();
    const timer = window.setTimeout(() => controller.abort(), timeoutMs);
    try {
      return await fetchJson(url, {
        ...options,
        signal: controller.signal
      });
    } catch (err) {
      if (err?.name === "AbortError") {
        throw new Error("选择器请求超时，请重试");
      }
      throw err;
    } finally {
      window.clearTimeout(timer);
    }
  }

  async function pickOutputDir() {
    try {
      appendLog("info", "正在打开输出目录选择器...");
      const data = await fetchJsonWithTimeout("/api/pick-directory", { method: "POST" });
      if (data.path) {
        outputDirInput.value = data.path;
        savePreferences();
        appendLog("success", `输出目录：${data.path}`);
        toastSuccess("输出目录已更新");
        emitStateChanged();
        updateConfigAutoCollapse();
      }
    } catch (err) {
      appendLog("error", `选择输出目录失败：${err.message}`);
      toastError("选择输出目录失败");
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
      const data = await fetchJsonWithTimeout("/api/pick-db-file", { method: "POST" });
      if (data.path) {
        dbPathInput.value = data.path;
        state.manualDbValid = true;
        setHintStatus(dbStatus, "success", `数据库路径：${data.path}`);
        savePreferences();
        appendLog("success", "KGMusicV3.db 已选择。");
        toastSuccess("数据库路径已更新");
      }
      emitStateChanged();
      updateConfigAutoCollapse();
    } catch (err) {
      appendLog("error", `选择数据库失败：${err.message}`);
      toastError("选择数据库失败");
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
        toastSuccess("数据库检测成功");
      } else {
        state.autoDbFound = false;
        if (state.manualDbValid) setHintStatus(dbStatus, "success", "当前使用手动配置数据库。");
        else setHintStatus(dbStatus, "warn", "仍未检测到 KGMusicV3.db。");
        appendLog("warn", "自动检测未找到 KGMusicV3.db。");
        toastInfo("未检测到数据库");
      }

      const config = await fetchJson("/api/config");
      applyConfig(config);
    } catch (err) {
      appendLog("error", `重新检测失败：${err.message}`);
      toastError("重新检测数据库失败");
    }
  }

  return {
    savePreferences,
    loadPreferences,
    applyTheme,
    initTheme,
    toggleTheme,
    updateMp3QualityVisibility,
    buildConfigSummaryText,
    setConfigCollapsed,
    updateConfigAutoCollapse,
    applyConfig,
    loadConfig,
    validateDbPath,
    scheduleDbValidation,
    pickOutputDir,
    openOutputFolder,
    pickDbFile,
    redetectDb
  };
}
