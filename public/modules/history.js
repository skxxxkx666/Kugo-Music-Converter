export function loadHistory(key) {
  try {
    const parsed = JSON.parse(localStorage.getItem(key) || "[]");
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export function appendHistory(history, summary, fallback = {}) {
  const next = Array.isArray(history) ? [...history] : [];
  next.unshift({
    timestamp: new Date().toISOString(),
    total: summary?.total || 0,
    success: summary?.success || 0,
    failed: summary?.failed || 0,
    durationMs: summary?.durationMs || 0,
    outputDir: summary?.outputDir || fallback.outputDir || "",
    outputFormat: summary?.outputFormat || fallback.outputFormat || ""
  });
  if (next.length > 50) next.length = 50;
  return next;
}

export function saveHistory(key, history) {
  localStorage.setItem(key, JSON.stringify(Array.isArray(history) ? history : []));
}

export function renderHistoryPanel(panel, history, options = {}) {
  if (!panel) return;
  const {
    escapeHtml = (v) => String(v ?? ""),
    formatDuration = (v) => String(v ?? ""),
    limit = 10,
    expanded = false,
    onRestore = null,
    onDelete = null,
    onToggleExpand = null
  } = options;

  panel.innerHTML = "";
  const list = Array.isArray(history) ? history : [];
  if (list.length === 0) {
    panel.innerHTML = '<div class="history-empty">暂无历史记录</div>';
    return;
  }

  const showAll = Boolean(expanded);
  const visible = showAll ? list : list.slice(0, limit);

  visible.forEach((item, index) => {
    const row = document.createElement("div");
    row.className = "history-item";
    row.setAttribute("role", "listitem");

    const timeText = new Date(item.timestamp).toLocaleString("zh-CN", { hour12: false });
    const outputFormat = String(item.outputFormat || "").toUpperCase() || "COPY";
    const outputDir = String(item.outputDir || "").trim() || "未记录输出目录";
    row.innerHTML = `
      <div class="history-main">${escapeHtml(timeText)}</div>
      <div class="history-sub">文件 ${escapeHtml(item.total)} | 成功 ${escapeHtml(item.success)} | 失败 ${escapeHtml(item.failed)} | ${escapeHtml(formatDuration(item.durationMs))} | ${escapeHtml(outputFormat)}</div>
      <div class="history-sub history-path" title="${escapeHtml(outputDir)}">${escapeHtml(outputDir)}</div>
      <div class="history-actions">
        <button type="button" class="btn-secondary history-restore-btn" data-index="${index}" aria-label="恢复此条历史配置">
          恢复配置
        </button>
        <button type="button" class="btn-secondary history-delete-btn" data-index="${index}" aria-label="删除此条历史记录">
          删除
        </button>
      </div>
    `;

    if (typeof onRestore === "function") {
      const restoreBtn = row.querySelector(".history-restore-btn");
      restoreBtn?.addEventListener("click", (event) => {
        event.stopPropagation();
        onRestore(index, item);
      });
      row.addEventListener("click", (event) => {
        if (event.target.closest(".history-delete-btn")) return;
        onRestore(index, item);
      });
    }
    if (typeof onDelete === "function") {
      const deleteBtn = row.querySelector(".history-delete-btn");
      deleteBtn?.addEventListener("click", (event) => {
        event.stopPropagation();
        onDelete(index, item);
      });
    }

    panel.appendChild(row);
  });

  if (list.length > limit && typeof onToggleExpand === "function") {
    const toggleRow = document.createElement("div");
    toggleRow.className = "history-toggle-row";

    const toggleBtn = document.createElement("button");
    toggleBtn.type = "button";
    toggleBtn.className = "btn-secondary history-toggle-btn";
    toggleBtn.textContent = showAll ? "收起" : `查看更多（${list.length - limit}）`;
    toggleBtn.setAttribute("aria-label", showAll ? "收起历史记录" : "展开查看更多历史记录");
    toggleBtn.addEventListener("click", () => onToggleExpand(!showAll));

    toggleRow.appendChild(toggleBtn);
    panel.appendChild(toggleRow);
  }
}
