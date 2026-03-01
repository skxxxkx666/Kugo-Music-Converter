export function createConverterController(options) {
  const {
    state,
    elements,
    helpers,
    callbacks
  } = options;

  const {
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
  } = elements;

  const {
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
  } = helpers;

  const {
    getPendingItems,
    requiresDb,
    isDbReady,
    getSelectedFiles,
    getPathQueue,
    setBusy,
    onQueueChanged,
    queueFailedResults,
    queueRetrySnapshotItems,
    onConvertComplete,
    announceCompletion
  } = callbacks;

  let pendingSseEvents = [];
  let sseFramePending = false;
  let successResults = [];
  let activePreviewIndex = -1;
  let lastSubmittedSnapshot = [];

  function resetPreviewPlayer() {
    activePreviewIndex = -1;
    if (previewAudio) {
      previewAudio.pause();
      previewAudio.removeAttribute("src");
      previewAudio.load();
      previewAudio.classList.add("hidden");
    }
    if (!successList) return;
    successList.querySelectorAll(".success-item").forEach((node) => {
      node.classList.remove("active-preview");
    });
  }

  function updateSuccessPreviewActive() {
    if (!successList) return;
    successList.querySelectorAll(".success-item").forEach((node) => {
      const index = Number.parseInt(node.getAttribute("data-index"), 10);
      node.classList.toggle("active-preview", index === activePreviewIndex);
    });
  }

  if (previewAudio) {
    previewAudio.addEventListener("ended", () => {
      activePreviewIndex = -1;
      updateSuccessPreviewActive();
    });
  }

  function updateProgressBar(percent, tone = "normal") {
    const safe = Math.max(0, Math.min(100, Number(percent) || 0));
    if (hasGSAP() && !prefersReducedMotion()) {
      window.gsap.to(totalProgressBar, {
        width: `${safe}%`,
        duration: 0.4,
        ease: "power1.out",
        overwrite: "auto"
      });
      if (safe >= 100 && !state.progressPulseDone) {
        state.progressPulseDone = true;
        window.gsap.fromTo(
          totalProgressBar,
          { scale: 1, transformOrigin: "center center" },
          { scale: 1.04, duration: 0.14, ease: "power1.out", repeat: 1, yoyo: true }
        );
      }
    } else {
      totalProgressBar.style.width = `${safe}%`;
    }
    totalProgressText.textContent = `${safe}%`;
    totalProgressBar.classList.toggle("warning", tone === "warning");
    totalProgressBar.classList.toggle("error", tone === "error");
    totalProgressBar.setAttribute("aria-valuenow", String(safe));
  }

  function resolveCompletionTone(summary) {
    const success = Math.max(0, Number(summary?.success) || 0);
    const failed = Math.max(0, Number(summary?.failed) || 0);
    if (failed <= 0) return "normal";
    if (success <= 0) return "error";
    return "warning";
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

  function animateResultDashboardReveal() {
    if (resultDashboard.classList.contains("hidden")) return;
    if (!hasGSAP() || prefersReducedMotion()) return;
    fadeIn(resultDashboard, { duration: 0.4, ease: "power2.out" });
  }

  function renderDashboard(summary) {
    function animateStatNumber(targetEl, targetValue, formatter) {
      const safeValue = Math.max(0, Number(targetValue) || 0);
      const render = typeof formatter === "function" ? formatter : (value) => String(value);
      targetEl.textContent = render(0);

      if (!hasGSAP() || prefersReducedMotion()) {
        targetEl.textContent = render(safeValue);
        return;
      }

      const counter = { value: 0 };
      window.gsap.to(counter, {
        value: safeValue,
        duration: 0.95,
        ease: "power2.out",
        snap: { value: 1 },
        onUpdate: () => {
          targetEl.textContent = render(Math.round(counter.value));
        }
      });
    }

    resultDashboard.classList.remove("hidden");
    animateStatNumber(statSuccess, summary.success || 0, (value) => String(value));
    animateStatNumber(statFailed, summary.failed || 0, (value) => String(value));
    animateStatNumber(statDuration, Math.round((summary.durationMs || 0) / 1000), (value) => formatDuration(value * 1000));
    animateResultDashboardReveal();
  }

  function renderFailedDetails(results) {
    const failed = (results || []).filter((item) => item?.status === "error");
    state.failedResults = failed;
    failedList.innerHTML = "";
    if (retryFailedBtn) retryFailedBtn.disabled = failed.length === 0;

    if (failed.length === 0) {
      failedSection.classList.add("hidden");
      return;
    }

    failed.forEach((item, idx) => {
      const row = document.createElement("div");
      row.className = "failed-item";
      row.setAttribute("role", "listitem");
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
        <button class="btn-secondary retry-single-btn" type="button" data-index="${idx}" aria-label="重试该失败文件">
          ${iconMarkup("rotate-cw", "ui-icon", true)}<span>重试该文件</span>
        </button>
      `;
      failedList.appendChild(row);
    });

    failedSection.classList.remove("hidden");
    refreshIcons();
  }

  function renderSuccessDetails(results) {
    const succeeded = (results || []).filter((item) => item?.status === "ok" && item?.output);
    successResults = succeeded;
    if (!successList || !successSection) return;

    successList.innerHTML = "";
    resetPreviewPlayer();

    if (succeeded.length === 0) {
      successSection.classList.add("hidden");
      return;
    }

    succeeded.forEach((item, index) => {
      const row = document.createElement("div");
      row.className = "success-item";
      row.setAttribute("role", "listitem");
      row.setAttribute("data-index", String(index));
      row.innerHTML = `
        <div class="success-main">${escapeHtml(`${index + 1}. ${item.file || "未知文件"}`)}</div>
        <div class="success-path" title="${escapeHtml(item.output || "")}">${escapeHtml(item.output || "")}</div>
        <button class="btn-secondary preview-file-btn" type="button" data-index="${index}" aria-label="试听该输出文件">
          ${iconMarkup("play", "ui-icon", true)}<span>试听</span>
        </button>
      `;
      successList.appendChild(row);
    });

    successSection.classList.remove("hidden");
    refreshIcons();
  }

  async function previewSuccessAt(index) {
    if (!Number.isFinite(index) || index < 0 || index >= successResults.length) return;
    const item = successResults[index];
    const outputPath = String(item?.output || "").trim();
    if (!outputPath || !previewAudio) {
      appendLog("warn", "该文件没有可试听路径。");
      return;
    }

    if (activePreviewIndex === index && !previewAudio.paused) {
      previewAudio.pause();
      appendLog("info", `已暂停试听：${item.file}`);
      return;
    }

    try {
      const url = `/api/preview-file?path=${encodeURIComponent(outputPath)}&_ts=${Date.now()}`;
      if (activePreviewIndex !== index || previewAudio.src !== url) {
        previewAudio.src = url;
      }
      await previewAudio.play();
      activePreviewIndex = index;
      previewAudio.classList.remove("hidden");
      updateSuccessPreviewActive();
      appendLog("info", `正在试听：${item.file}`);
      toastInfo(`正在试听：${item.file}`);
    } catch (err) {
      appendLog("error", `试听失败：${err?.message || "无法播放该文件"}`);
    }
  }

  function handleSuccessListClick(event) {
    const btn = event.target.closest(".preview-file-btn");
    if (!btn) return;
    const index = Number.parseInt(btn.getAttribute("data-index"), 10);
    previewSuccessAt(index);
  }

  function retryFailedFiles(items = state.failedResults) {
    const failedItems = Array.isArray(items) ? items : [];
    const snapshotRetryItems = [];
    const unresolvedFailedItems = [];

    failedItems.forEach((item) => {
      const current = Number(item?.current);
      if (!Number.isFinite(current) || current <= 0 || current > lastSubmittedSnapshot.length) {
        unresolvedFailedItems.push(item);
        return;
      }
      const snapshotItem = lastSubmittedSnapshot[current - 1];
      if (!snapshotItem) {
        unresolvedFailedItems.push(item);
        return;
      }
      snapshotRetryItems.push(snapshotItem);
    });

    const addedFromSnapshot =
      typeof queueRetrySnapshotItems === "function" ? queueRetrySnapshotItems(snapshotRetryItems) : 0;
    const addedFromFallback =
      typeof queueFailedResults === "function" ? queueFailedResults(unresolvedFailedItems) : 0;
    const added = addedFromSnapshot + addedFromFallback;

    if (added > 0) {
      if (typeof onQueueChanged === "function") onQueueChanged();
      appendLog("success", `已将 ${added} 个失败文件重新加入队列。`);
      toastSuccess(`已加入 ${added} 个失败文件到队列`);
    } else if (snapshotRetryItems.length > 0) {
      appendLog("info", "失败文件已在当前队列中，可直接点击“开始转换”。");
      toastInfo("失败文件已在队列中");
    } else {
      appendLog("warn", "没有可重试的失败文件（可能为上传文件或已在队列中）。");
      toastInfo("没有可重试的失败文件");
    }
  }

  function retryFailedAt(index) {
    if (!Number.isFinite(index) || index < 0 || index >= state.failedResults.length) return;
    retryFailedFiles([state.failedResults[index]]);
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
    state.progressPulseDone = false;
    pendingSseEvents = [];
    sseFramePending = false;
    successResults = [];
    resetPreviewPlayer();

    if (successSection) successSection.classList.add("hidden");
    if (successList) successList.innerHTML = "";
    failedSection.classList.add("hidden");
    failedList.innerHTML = "";
    if (retryFailedBtn) retryFailedBtn.disabled = true;

    updateProgressBar(0, "normal");
    progressStatus.textContent = "准备开始转换...";
    progressETA.textContent = "";
    statSuccess.textContent = "0";
    statFailed.textContent = "0";
    statDuration.textContent = formatDuration(0);
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
    row.setAttribute("role", "listitem");

    const icon = document.createElement("span");
    icon.className = "status-icon";
    setStatusIcon(icon, "clock-3", "等待中");

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
    if (statusClass === "active") setStatusIcon(item.icon, "loader-circle", "进行中");
    if (statusClass === "success") setStatusIcon(item.icon, "circle-check", "转换成功");
    if (statusClass === "error") setStatusIcon(item.icon, "circle-x", "转换失败");
    if (statusClass === "pending") setStatusIcon(item.icon, "clock-3", "等待中");
    item.text.textContent = `[${data.current}/${data.total}] ${data.file} ${statusText}`;
  }

  function phaseText(phase) {
    if (phase === "prepare") return "准备中";
    if (phase === "decrypt") return "解密中";
    if (phase === "transcode") return "转码中";
    return "处理中";
  }

  function handleProgressEvent(eventName, data) {
    pendingSseEvents.push({ eventName, data });
    if (sseFramePending) return;
    sseFramePending = true;
    requestAnimationFrame(flushSseEvents);
  }

  function flushSseEvents() {
    sseFramePending = false;
    const queued = pendingSseEvents;
    pendingSseEvents = [];
    if (queued.length === 0) return;

    queued.forEach(({ eventName, data }) => {
      applyProgressEvent(eventName, data);
    });
    refreshIcons();
  }

  function applyProgressEvent(eventName, data) {
    if (eventName === "progress") {
      const processed = Number(data.bytesProcessed);
      const totalBytes = Number(data.totalBytes);
      if (Number.isFinite(processed) && Number.isFinite(totalBytes) && totalBytes > 0) {
        state.progressDone = Math.max(0, processed);
        state.progressTotal = Math.max(0, totalBytes);
        updateProgressETA();
      }
      progressStatus.textContent = `${phaseText(data.phase)}：${data.file} (${data.current}/${data.total})`;
      updateProgressBar(data.percent, state.hasFileError ? "warning" : "normal");
      updateFileRow(data, "active", `- ${phaseText(data.phase)}...`);
      return;
    }

    if (eventName === "file-done") {
      const processed = Number(data.bytesProcessed);
      const totalBytes = Number(data.totalBytes);
      if (Number.isFinite(processed) && Number.isFinite(totalBytes) && totalBytes > 0) {
        state.progressDone = Math.max(0, processed);
        state.progressTotal = Math.max(0, totalBytes);
      } else {
        state.progressDone += 1;
      }
      updateProgressETA();
      const hasIssue = state.hasFileError || data.status === "error";
      updateProgressBar(data.percent, hasIssue ? "warning" : "normal");

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
      updateProgressBar(100, resolveCompletionTone(data));
      progressStatus.textContent = doneText;
      progressETA.textContent = "";
      appendLog("info", doneText);
      if (data.cancelled) appendLog("warn", "任务已取消。");

      renderDashboard(data);
      renderSuccessDetails(data.results || []);
      renderFailedDetails(data.results || []);
      if (typeof announceCompletion === "function") announceCompletion(doneText);
      if (typeof onConvertComplete === "function") onConvertComplete(data);
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

    const uploadSnapshot = getSelectedFiles().slice();
    const pathSnapshot = getPathQueue().map((item) => ({ ...item }));
    lastSubmittedSnapshot = items.map((item) => {
      if (item.source === "upload") {
        const file = uploadSnapshot[item.index];
        if (!file) return null;
        return {
          source: "upload",
          file,
          name: file.name,
          size: file.size
        };
      }
      if (item.source === "path") {
        const pathItem = pathSnapshot[item.index];
        const fullPath = String(pathItem?.fullPath || item.fullPath || "").trim();
        if (!fullPath) return null;
        return {
          source: "path",
          fullPath,
          name: pathItem?.name || item.name || fullPath.split(/[\\/]/).pop() || fullPath,
          size: Number(pathItem?.size) || Number(item.size) || 0
        };
      }
      return null;
    });

    const formData = new FormData();
    formData.append("outputDir", outputDir);
    formData.append("outputFormat", outputFormatSelect.value);
    formData.append("mp3Quality", mp3QualitySelect.value);
    formData.append("concurrency", concurrencySelect.value);

    if (dbPath) formData.append("dbPath", dbPath);
    for (const file of getSelectedFiles()) formData.append("kggFiles", file, file.name);
    if (getPathQueue().length > 0) {
      formData.append("inputPaths", JSON.stringify(getPathQueue().map((item) => item.fullPath)));
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
      const confirmed = confirmDestructive("当前正在转换，确定要取消吗？");
      if (!confirmed) {
        appendLog("info", "已取消本次“取消转换”操作。");
        return;
      }
      state.abortController.abort();
      appendLog("warn", "用户已取消转换。");
      toastInfo("已取消转换任务");
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
    setTimeout(() => URL.revokeObjectURL(a.href), 10000);
    appendLog("success", "失败文件列表已导出。");
    toastSuccess("失败文件列表已导出");
  }

  return {
    startConvert,
    cancelConvert,
    exportFailedList,
    retryFailedFiles,
    retryFailedAt,
    handleSuccessListClick
  };
}
