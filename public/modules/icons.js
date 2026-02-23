export function createIconToolkit(options = {}) {
  const {
    escapeHtml = (v) => String(v ?? ""),
    extIconMap = {},
    fallbackCdns = []
  } = options;

  let lucideEnsurePromise = null;
  let iconRefreshPending = false;

  function hasLucide() {
    return Boolean(window.lucide && typeof window.lucide.createIcons === "function");
  }

  function loadExternalScript(src, timeoutMs = 4500) {
    return new Promise((resolve) => {
      const script = document.createElement("script");
      let settled = false;
      script.src = src;
      script.async = true;
      script.defer = true;
      script.setAttribute("data-lucide-fallback", src);

      const cleanup = (ok) => {
        if (settled) return;
        settled = true;
        resolve(ok);
      };

      const timer = window.setTimeout(() => cleanup(false), timeoutMs);
      script.onload = () => {
        window.clearTimeout(timer);
        cleanup(true);
      };
      script.onerror = () => {
        window.clearTimeout(timer);
        cleanup(false);
      };

      document.head.appendChild(script);
    });
  }

  async function ensureLucideAvailable() {
    if (hasLucide()) return true;
    if (lucideEnsurePromise) return lucideEnsurePromise;

    lucideEnsurePromise = (async () => {
      for (const src of fallbackCdns) {
        const loaded = await loadExternalScript(src);
        if (!loaded) continue;
        if (hasLucide()) return true;
      }
      return false;
    })();

    const ok = await lucideEnsurePromise;
    if (!ok) lucideEnsurePromise = null;
    return ok;
  }

  function applyIconFallback() {
    const pending = document.querySelectorAll("i[data-lucide]");
    pending.forEach((node) => {
      if (node.querySelector("svg")) return;
      const className = node.getAttribute("class") || "ui-icon";
      node.innerHTML = `
        <svg class="${escapeHtml(className)}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="8"></circle>
          <path d="M12 8v8"></path>
          <path d="M8 12h8"></path>
        </svg>
      `;
    });
  }

  function refreshIcons() {
    if (iconRefreshPending) return;
    iconRefreshPending = true;

    requestAnimationFrame(() => {
      iconRefreshPending = false;

      if (hasLucide()) {
        window.lucide.createIcons({
          attrs: {
            "stroke-width": 1.9
          }
        });
        return;
      }

      applyIconFallback();
      ensureLucideAvailable().then((ok) => {
        if (ok && hasLucide()) {
          window.lucide.createIcons({
            attrs: {
              "stroke-width": 1.9
            }
          });
          return;
        }
        applyIconFallback();
      });
    });
  }

  function iconMarkup(name, className = "ui-icon", decorative = true, ariaLabel = "") {
    const attrs = decorative
      ? 'aria-hidden="true" focusable="false"'
      : `role="img" aria-label="${escapeHtml(ariaLabel)}"`;
    return `<i data-lucide="${escapeHtml(name)}" class="${escapeHtml(className)}" ${attrs}></i>`;
  }

  function extIconName(ext) {
    return extIconMap[String(ext || "").toLowerCase()] || "file";
  }

  function extBadgeClass(ext) {
    const normalized = String(ext || "")
      .replace(".", "")
      .toLowerCase()
      .replace(/[^a-z0-9_-]/g, "");
    return `file-ext-badge ext-${normalized || "unknown"}`;
  }

  function renderExtBadge(ext) {
    const extText = (ext || ".").replace(".", "").toUpperCase();
    return `
      <span class="${extBadgeClass(ext)}">
        ${iconMarkup(extIconName(ext), "ext-icon", true)}
        <span>${escapeHtml(extText)}</span>
      </span>
    `;
  }

  function setButtonContent(button, text, iconName, options2 = {}) {
    if (!button) return;
    const { iconOnly = false } = options2;
    const safeText = String(text || "").trim();
    const safeIcon = String(iconName || "").trim();
    button.innerHTML = "";

    if (!safeIcon) {
      button.textContent = safeText;
      if (safeText && !button.getAttribute("aria-label")) button.setAttribute("aria-label", safeText);
      return;
    }

    const content = document.createElement("span");
    content.className = "btn-content";
    content.innerHTML = iconMarkup(safeIcon, "ui-icon", true);

    if (!iconOnly) {
      const textEl = document.createElement("span");
      textEl.className = "btn-text";
      textEl.textContent = safeText;
      content.appendChild(textEl);
    }

    button.appendChild(content);
    if (safeText && !button.getAttribute("aria-label")) button.setAttribute("aria-label", safeText);
  }

  function setStatusIcon(iconElement, iconName, ariaLabel) {
    if (!iconElement) return;
    if (iconElement.dataset.iconName === iconName) {
      iconElement.setAttribute("aria-label", ariaLabel);
      return;
    }
    iconElement.dataset.iconName = iconName;
    iconElement.innerHTML = iconMarkup(iconName, iconName === "loader-circle" ? "ui-icon spin" : "ui-icon", true);
    iconElement.setAttribute("aria-label", ariaLabel);
  }

  return {
    refreshIcons,
    iconMarkup,
    extIconName,
    extBadgeClass,
    renderExtBadge,
    setButtonContent,
    setStatusIcon
  };
}
