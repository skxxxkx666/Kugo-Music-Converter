const ENCRYPTED_EXTS = new Set([".kgg", ".kgm", ".kgma", ".vpr", ".ncm"]);

function csvEscape(value) {
  return `"${String(value ?? "").replace(/"/g, '""')}"`;
}

export function createScanner(ctx) {
  const {
    state,
    elements,
    fetchJson,
    appendLog,
    appendPayloadError,
    formatBytes,
    extBadgeClass,
    copyTextToClipboard,
    onQueueChanged,
    pendingCount
  } = ctx;

  function renderFolderTags() {
    elements.selectedFolders.innerHTML = "";
    state.selectedFolderPaths.forEach((folderPath, index) => {
      const tag = document.createElement("span");
      tag.className = "folder-tag";
      tag.innerHTML = `${folderPath}<button class="tag-remove" data-index="${index}">x</button>`;
      elements.selectedFolders.appendChild(tag);
    });
    elements.scanBtn.disabled = state.selectedFolderPaths.length === 0 || state.isBusy;
  }

  function removeFolder(index) {
    if (!Number.isFinite(index) || index < 0 || index >= state.selectedFolderPaths.length) return;
    state.selectedFolderPaths.splice(index, 1);
    renderFolderTags();
  }

  async function pickFolderForScan() {
    try {
      const data = await fetchJson("/api/pick-directory", { method: "POST" });
      if (data.path && !state.selectedFolderPaths.includes(data.path)) {
        state.selectedFolderPaths.push(data.path);
        renderFolderTags();
        appendLog("success", `已添加扫描目录：${data.path}`);
      }
    } catch (err) {
      appendLog("error", `选择文件夹失败：${err.message}`);
    }
  }

  function getScanFilterValue() {
    return elements.extFilter.value === "custom" ? elements.customExtFilter.value.trim() : elements.extFilter.value;
  }

  function renderScanResult(data) {
    elements.scanResult.classList.remove("hidden");
    elements.scanTotal.textContent = `${data.totalFiles || 0} 个文件`;
    elements.scanSize.textContent = formatBytes(data.totalSize || 0);
    elements.fileNameList.innerHTML = "";

    const all = [];
    (data.folders || []).forEach((folder) => {
      const header = document.createElement("div");
      header.className = "folder-header";
      header.textContent = `${folder.path}（${(folder.files || []).length} 个文件）`;
      elements.fileNameList.appendChild(header);

      (folder.files || []).forEach((file) => {
        all.push(file);
        const row = document.createElement("div");
        row.className = "file-name-item";
        row.innerHTML = `
          <span class="${extBadgeClass(file.ext)}">${String(file.ext || "").replace(".", "").toUpperCase()}</span>
          <span class="file-name-col" title="${file.fullPath || ""}">${file.name}</span>
          <span class="file-size-col">${formatBytes(file.size)}</span>
          <span class="file-date-col">${new Date(file.modTime).toLocaleString("zh-CN", { hour12: false })}</span>
        `;
        elements.fileNameList.appendChild(row);
      });
    });

    state.scanFiles = all;
  }

  async function startScanFolders() {
    if (state.selectedFolderPaths.length === 0) return;

    try {
      elements.scanBtn.disabled = true;
      appendLog("info", "开始扫描文件夹...");
      const data = await fetchJson("/api/scan-folders", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          paths: state.selectedFolderPaths,
          recursive: elements.scanRecursive.checked,
          filter: getScanFilterValue()
        })
      });
      renderScanResult(data);
      appendLog("success", `扫描完成：共 ${data.totalFiles || 0} 个文件`);
    } catch (err) {
      appendPayloadError("扫描失败：", err.payload || { userMessage: err.message });
    } finally {
      elements.scanBtn.disabled = state.selectedFolderPaths.length === 0 || state.isBusy;
    }
  }

  function copyNames() {
    const names = state.scanFiles.map((file) => file.name).join("\n");
    copyTextToClipboard(names, `已复制 ${state.scanFiles.length} 个文件名到剪贴板`);
  }

  function copyPaths() {
    const paths = state.scanFiles.map((file) => file.fullPath).join("\n");
    copyTextToClipboard(paths, `已复制 ${state.scanFiles.length} 个完整路径到剪贴板`);
  }

  function exportCsv() {
    if (state.scanFiles.length === 0) {
      appendLog("warn", "暂无扫描结果可导出。");
      return;
    }

    const rows = [
      "文件名,扩展名,大小(Byte),修改时间,完整路径",
      ...state.scanFiles.map(
        (file) =>
          `${csvEscape(file.name)},${csvEscape(file.ext || "")},${csvEscape(file.size || 0)},${csvEscape(file.modTime || "")},${csvEscape(file.fullPath || "")}`
      )
    ];

    const blob = new Blob(["\uFEFF" + rows.join("\n")], { type: "text/csv;charset=utf-8" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = `文件名列表_${new Date().toISOString().slice(0, 10)}.csv`;
    a.click();
    URL.revokeObjectURL(a.href);
    appendLog("success", "CSV 已导出。");
  }

  function addScanFilesToQueue() {
    const candidates = state.scanFiles.filter((file) => ENCRYPTED_EXTS.has(String(file.ext || "").toLowerCase()));
    if (candidates.length === 0) {
      appendLog("warn", "扫描结果中没有可转换的加密音频文件。");
      return;
    }

    const existed = new Set(state.pathQueue.map((item) => item.fullPath));
    let added = 0;

    candidates.forEach((file) => {
      if (!file.fullPath || existed.has(file.fullPath)) return;
      if (pendingCount() + added >= state.maxFileCount) return;

      state.pathQueue.push({
        fullPath: file.fullPath,
        name: file.name,
        size: file.size || 0,
        ext: String(file.ext || "").toLowerCase()
      });
      existed.add(file.fullPath);
      added += 1;
    });

    if (added > 0) {
      onQueueChanged();
      appendLog("success", `已将 ${added} 个文件加入转换队列。`);
    } else {
      appendLog("warn", "未新增文件（可能已存在或超过上限）。");
    }
  }

  function handleExtFilterChange() {
    elements.customExtWrap.classList.toggle("hidden", elements.extFilter.value !== "custom");
  }

  return {
    renderFolderTags,
    removeFolder,
    pickFolderForScan,
    startScanFolders,
    copyNames,
    copyPaths,
    exportCsv,
    addScanFilesToQueue,
    handleExtFilterChange
  };
}
