export function createToastManager(options = {}) {
  const host = options.host || document.body;
  const maxVisible = Math.max(1, Number(options.maxVisible) || 3);
  const defaultDuration = Math.max(1200, Number(options.defaultDuration) || 3000);

  function show(message, type = "info", duration = defaultDuration) {
    if (!host) return;
    const text = String(message || "").trim();
    if (!text) return;

    const toast = document.createElement("div");
    toast.className = `toast toast-enter toast-${normalizeToastType(type)}`;
    toast.setAttribute("role", "status");
    toast.textContent = text;
    host.appendChild(toast);

    trimToasts(host, maxVisible);
    requestAnimationFrame(() => {
      toast.classList.add("show");
    });

    const life = Math.max(1200, Number(duration) || defaultDuration);
    const timer = window.setTimeout(() => removeToast(toast), life);
    toast.addEventListener(
      "click",
      () => {
        window.clearTimeout(timer);
        removeToast(toast);
      },
      { once: true }
    );
  }

  return {
    show,
    success(message, duration) {
      show(message, "success", duration);
    },
    error(message, duration) {
      show(message, "error", duration);
    },
    info(message, duration) {
      show(message, "info", duration);
    }
  };
}

function trimToasts(host, maxVisible) {
  const toasts = host.querySelectorAll(".toast");
  if (toasts.length <= maxVisible) return;
  const removeCount = toasts.length - maxVisible;
  for (let i = 0; i < removeCount; i += 1) {
    removeToast(toasts[i]);
  }
}

function normalizeToastType(type) {
  const normalized = String(type || "info").toLowerCase();
  if (normalized === "success" || normalized === "error" || normalized === "info") return normalized;
  return "info";
}

function removeToast(toast) {
  if (!toast || !toast.parentNode) return;
  if (toast.dataset.removing === "1") return;

  toast.dataset.removing = "1";
  toast.classList.remove("show");
  toast.classList.remove("toast-enter");
  toast.classList.add("toast-exit");

  let cleaned = false;
  const cleanup = () => {
    if (cleaned) return;
    cleaned = true;
    if (toast.parentNode) toast.parentNode.removeChild(toast);
  };

  toast.addEventListener("animationend", cleanup, { once: true });
  window.setTimeout(cleanup, 320);
}

export async function withButtonLoading(button, runTask, options = {}) {
  if (!button || typeof runTask !== "function") return runTask?.();

  const {
    setButtonContent,
    loadingText = "处理中...",
    loadingIcon = "loader-circle",
    recoverText = button.textContent?.trim() || "",
    recoverIcon = button.getAttribute("data-icon") || ""
  } = options;

  const previousDisabled = button.disabled;
  button.disabled = true;

  if (typeof setButtonContent === "function") {
    setButtonContent(button, loadingText, loadingIcon);
  } else {
    button.textContent = loadingText;
  }

  try {
    return await runTask();
  } finally {
    button.disabled = previousDisabled;
    if (typeof setButtonContent === "function") {
      setButtonContent(button, recoverText, recoverIcon);
    } else {
      button.textContent = recoverText;
    }
  }
}

export function confirmDestructive(message) {
  const text = String(message || "").trim();
  if (!text) return true;
  return window.confirm(text);
}

