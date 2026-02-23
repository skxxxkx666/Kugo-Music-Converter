import {
  appendHistory,
  loadHistory,
  renderHistoryPanel,
  saveHistory
} from "./history.js";

export function createHistoryController(options) {
  const {
    storageKey,
    panel,
    escapeHtml,
    formatDuration,
    appendLog,
    toastInfo,
    onRestore
  } = options;

  let history = [];
  let expanded = false;

  function render() {
    renderHistoryPanel(panel, history, {
      escapeHtml,
      formatDuration,
      limit: 10,
      expanded,
      onRestore: (index, item) => {
        if (typeof onRestore === "function") onRestore(index, item);
      },
      onDelete: (index) => {
        removeAt(index);
      },
      onToggleExpand: (next) => {
        expanded = Boolean(next);
        render();
      }
    });
  }

  function load() {
    history = loadHistory(storageKey);
  }

  function append(summary, fallback = {}) {
    history = appendHistory(history, summary, fallback);
    saveHistory(storageKey, history);
    render();
  }

  function clear() {
    history = [];
    expanded = false;
    localStorage.removeItem(storageKey);
    render();
  }

  function removeAt(index) {
    if (!Number.isFinite(index) || index < 0 || index >= history.length) return;
    history.splice(index, 1);
    if (history.length <= 10) expanded = false;
    saveHistory(storageKey, history);
    render();
    if (typeof appendLog === "function") appendLog("info", "已删除一条历史记录。");
    if (typeof toastInfo === "function") toastInfo("历史记录已删除");
  }

  function restoreAt(index, item) {
    if (typeof onRestore !== "function") return;
    const target = item || history[index];
    if (!target) return;
    onRestore(index, target);
  }

  function getAll() {
    return [...history];
  }

  return {
    load,
    render,
    append,
    clear,
    removeAt,
    restoreAt,
    getAll
  };
}
