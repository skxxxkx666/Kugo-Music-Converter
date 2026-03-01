const SUPPORTED_QUEUE_EXTS = [".kgg", ".kgm", ".kgma", ".vpr", ".ncm"];
const VIRTUAL_LIST_THRESHOLD = 120;
const VIRTUAL_ROW_HEIGHT = 48;
const VIRTUAL_OVERSCAN = 6;

export function createUploaderController(options) {
  const { state, elements, helpers } = options;
  const { filePreviewList, fileSummary, clearQueueBtn } = elements;
  const { getExt, formatBytes, renderExtBadge, escapeHtml, setHintStatus, appendLog, refreshIcons } = helpers;

  let uploadFileIdSeed = 1;
  const uploadFileIdMap = new WeakMap();

  const virtual = {
    enabled: false,
    items: [],
    scrollBound: false,
    rafPending: false
  };

  function getUploadFileKey(file) {
    if (!uploadFileIdMap.has(file)) {
      uploadFileIdMap.set(file, `upload:${uploadFileIdSeed++}`);
    }
    return uploadFileIdMap.get(file);
  }

  function getPendingItems() {
    const uploadItems = state.selectedFiles.map((file, index) => ({
      source: "upload",
      key: getUploadFileKey(file),
      index,
      name: file.name,
      size: file.size,
      ext: getExt(file.name)
    }));
    const pathItems = state.pathQueue.map((item, index) => ({
      source: "path",
      key: `path:${String(item.fullPath || "")}`,
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

  function updateFilePreviewRow(row, item) {
    if (!row) return;
    const titleText = item.source === "path" ? item.fullPath : item.name;
    const prefix = item.source === "path" ? "[路径]" : "[上传]";
    const displayName = `${prefix} ${item.name}`;
    row.innerHTML = `
      ${renderExtBadge(item.ext)}
      <span class="file-preview-name" title="${escapeHtml(titleText)}">
        ${escapeHtml(displayName)}
      </span>
      <span class="file-preview-size">${formatBytes(item.size)}</span>
      <button class="btn-remove" type="button" data-source="${item.source}" data-index="${item.index}" aria-label="移除文件">
        <i data-lucide="trash-2" class="ui-icon" aria-hidden="true" focusable="false"></i>
      </button>
    `;
  }

  function buildPreviewRow(item) {
    const row = document.createElement("div");
    row.className = "file-preview-item";
    row.setAttribute("role", "listitem");
    updateFilePreviewRow(row, item);
    return row;
  }

  function ensureVirtualScrollBinding() {
    if (virtual.scrollBound) return;
    virtual.scrollBound = true;

    const schedule = () => {
      if (!virtual.enabled || virtual.rafPending) return;
      virtual.rafPending = true;
      requestAnimationFrame(() => {
        virtual.rafPending = false;
        renderVirtualViewport();
      });
    };

    filePreviewList.addEventListener("scroll", schedule);
    window.addEventListener("resize", schedule);
  }

  function renderVirtualViewport() {
    const items = virtual.items;
    if (!virtual.enabled || !Array.isArray(items) || items.length === 0) return;

    const viewportHeight = Math.max(filePreviewList.clientHeight || 260, VIRTUAL_ROW_HEIGHT);
    const scrollTop = Math.max(0, filePreviewList.scrollTop);

    const start = Math.max(0, Math.floor(scrollTop / VIRTUAL_ROW_HEIGHT) - VIRTUAL_OVERSCAN);
    const visibleCount = Math.ceil(viewportHeight / VIRTUAL_ROW_HEIGHT) + VIRTUAL_OVERSCAN * 2;
    const end = Math.min(items.length, start + visibleCount);

    const fragment = document.createDocumentFragment();

    const topSpacer = document.createElement("div");
    topSpacer.className = "file-preview-spacer";
    topSpacer.style.height = `${start * VIRTUAL_ROW_HEIGHT}px`;
    fragment.appendChild(topSpacer);

    for (let index = start; index < end; index += 1) {
      fragment.appendChild(buildPreviewRow(items[index]));
    }

    const bottomSpacer = document.createElement("div");
    bottomSpacer.className = "file-preview-spacer";
    bottomSpacer.style.height = `${Math.max(0, (items.length - end) * VIRTUAL_ROW_HEIGHT)}px`;
    fragment.appendChild(bottomSpacer);

    filePreviewList.innerHTML = "";
    filePreviewList.appendChild(fragment);
    refreshIcons();
  }

  function renderNormalList(items) {
    virtual.enabled = false;
    virtual.items = [];
    filePreviewList.classList.remove("virtual-list");

    if (items.length === 0) {
      filePreviewList.innerHTML = "";
      state.filePreviewRowMap.clear();
      return;
    }

    const nextKeys = new Set(items.map((item) => item.key));
    Array.from(state.filePreviewRowMap.entries()).forEach(([key, row]) => {
      if (nextKeys.has(key)) return;
      row.remove();
      state.filePreviewRowMap.delete(key);
    });

    const fragment = document.createDocumentFragment();
    items.forEach((item) => {
      let row = state.filePreviewRowMap.get(item.key);
      if (!row) {
        row = document.createElement("div");
        row.className = "file-preview-item";
        row.setAttribute("role", "listitem");
        state.filePreviewRowMap.set(item.key, row);
      }
      updateFilePreviewRow(row, item);
      fragment.appendChild(row);
    });

    filePreviewList.innerHTML = "";
    filePreviewList.appendChild(fragment);
    refreshIcons();
  }

  function renderVirtualList(items) {
    ensureVirtualScrollBinding();
    virtual.enabled = true;
    virtual.items = items;
    filePreviewList.classList.add("virtual-list");
    state.filePreviewRowMap.clear();
    renderVirtualViewport();
  }

  function renderFilePreview() {
    const items = getPendingItems();
    if (items.length > VIRTUAL_LIST_THRESHOLD) {
      renderVirtualList(items);
      return;
    }
    renderNormalList(items);
  }

  function updateFileSummary() {
    const items = getPendingItems();
    if (items.length === 0) {
      setHintStatus(fileSummary, "", "未选择文件");
      if (clearQueueBtn) clearQueueBtn.disabled = true;
      return;
    }

    const totalBytes = items.reduce((sum, item) => sum + (item.size || 0), 0);
    const extStats = new Map();
    items.forEach((item) => {
      const ext = String(item.ext || "").toLowerCase() || "unknown";
      extStats.set(ext, (extStats.get(ext) || 0) + 1);
    });
    const statsText = Array.from(extStats.entries())
      .sort((a, b) => b[1] - a[1])
      .map(([ext, count]) => `${ext.replace(".", "").toUpperCase()}:${count}`)
      .join(" | ");

    if (items.length > state.maxFileCount) {
      setHintStatus(fileSummary, "error", `已选择 ${items.length} 个文件（超过上限 ${state.maxFileCount}）`);
      if (clearQueueBtn) clearQueueBtn.disabled = false;
      return;
    }

    setHintStatus(
      fileSummary,
      "success",
      `已选择 ${items.length} 个文件，总大小 ${formatBytes(totalBytes)}（上传 ${state.selectedFiles.length}，路径队列 ${state.pathQueue.length}） | ${statsText}`
    );
    if (clearQueueBtn) clearQueueBtn.disabled = false;
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
  }

  function clearQueue() {
    state.selectedFiles = [];
    state.pathQueue = [];
  }

  function removeAt(source, index) {
    if (!Number.isFinite(index)) return;
    if (source === "upload" && index >= 0 && index < state.selectedFiles.length) {
      state.selectedFiles.splice(index, 1);
      return;
    }
    if (source === "path" && index >= 0 && index < state.pathQueue.length) {
      state.pathQueue.splice(index, 1);
    }
  }

  function queueFailedResults(failedItems) {
    const list = Array.isArray(failedItems) ? failedItems : [];
    if (list.length === 0) return 0;

    const existed = new Set(state.pathQueue.map((item) => item.fullPath));
    let added = 0;
    list.forEach((item) => {
      const fullPath = String(item?.input || "").trim();
      if (!fullPath) return;
      if (!/^[a-zA-Z]:\\|^\//.test(fullPath)) return;
      if (existed.has(fullPath)) return;
      const ext = getExt(fullPath);
      if (!SUPPORTED_QUEUE_EXTS.includes(ext)) return;
      if (pendingCount() + added >= state.maxFileCount) return;

      state.pathQueue.push({
        fullPath,
        name: item.file || fullPath.split(/[\\/]/).pop() || fullPath,
        size: 0,
        ext
      });
      existed.add(fullPath);
      added += 1;
    });
    return added;
  }

  function queueRetrySnapshotItems(snapshotItems) {
    const list = Array.isArray(snapshotItems) ? snapshotItems : [];
    if (list.length === 0) return 0;

    const uploadSignatures = new Set(state.selectedFiles.map((file) => `${file.name}|${file.size}|${file.lastModified}`));
    const existedPathSet = new Set(state.pathQueue.map((item) => item.fullPath));

    let added = 0;
    list.forEach((item) => {
      if (pendingCount() + added >= state.maxFileCount) return;

      if (item?.source === "upload") {
        const file = item.file;
        if (!file || typeof file.name !== "string") return;
        const ext = getExt(file.name);
        if (!state.supportedFormats.includes(ext)) return;
        if (file.size > state.maxFileSizeMB * 1024 * 1024) return;

        const sign = `${file.name}|${file.size}|${file.lastModified}`;
        if (uploadSignatures.has(sign)) return;

        state.selectedFiles.push(file);
        uploadSignatures.add(sign);
        added += 1;
        return;
      }

      if (item?.source === "path") {
        const fullPath = String(item.fullPath || "").trim();
        if (!fullPath) return;
        if (!/^[a-zA-Z]:\\|^\//.test(fullPath)) return;
        if (existedPathSet.has(fullPath)) return;

        const ext = getExt(fullPath);
        if (!SUPPORTED_QUEUE_EXTS.includes(ext)) return;

        state.pathQueue.push({
          fullPath,
          name: item.name || fullPath.split(/[\\/]/).pop() || fullPath,
          size: Number(item.size) || 0,
          ext
        });
        existedPathSet.add(fullPath);
        added += 1;
      }
    });

    return added;
  }

  return {
    getPendingItems,
    pendingCount,
    requiresDb,
    isDbReady,
    renderFilePreview,
    updateFileSummary,
    mergeFiles,
    clearQueue,
    removeAt,
    queueFailedResults,
    queueRetrySnapshotItems
  };
}
