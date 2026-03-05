const UPDATE_CHECK_KEY = "kgg-converter-update-cache-v1";
const UPDATE_IGNORE_KEY = "kgg-converter-update-ignore-v1";
const UPDATE_CHECK_INTERVAL_MS = 24 * 60 * 60 * 1000;

// Primary: backend proxy (bypasses browser-level network restrictions)
const BACKEND_UPDATE_API = "/api/check-update";
// Fallback: direct GitHub API mirrors (for when backend proxy also fails)
const GITHUB_FALLBACK_URLS = [
  "https://api.github.com/repos/skxxxkx666/Kugo-Music-Converter/releases/latest",
  "https://ghfast.top/https://api.github.com/repos/skxxxkx666/Kugo-Music-Converter/releases/latest",
  "https://gh-proxy.com/https://api.github.com/repos/skxxxkx666/Kugo-Music-Converter/releases/latest"
];

export function createUpdateController(options) {
  const {
    appVersion,
    updateBannerHost,
    parseVersion,
    shouldNotifyUpdate,
    iconMarkup,
    escapeHtml,
    refreshIcons,
    hasGSAP,
    prefersReducedMotion,
    slideDown
  } = options;

  function summarizeReleaseBody(body) {
    const text = String(body || "")
      .replace(/\r/g, "")
      .split("\n")
      .map((line) => line.trim())
      .find((line) => line && !line.startsWith("#"));
    if (!text) return "包含功能改进与问题修复。";
    return text.length > 90 ? `${text.slice(0, 90)}...` : text;
  }

  function formatReleaseDate(value) {
    const date = new Date(value || "");
    if (Number.isNaN(date.getTime())) return "未知日期";
    return date.toLocaleDateString("zh-CN");
  }

  function readUpdateCache() {
    try {
      return JSON.parse(localStorage.getItem(UPDATE_CHECK_KEY) || "null");
    } catch {
      return null;
    }
  }

  function writeUpdateCache(data) {
    try {
      localStorage.setItem(
        UPDATE_CHECK_KEY,
        JSON.stringify({
          checkedAt: Date.now(),
          data
        })
      );
    } catch {
      // ignore storage failures
    }
  }

  function fetchWithTimeout(url, options, timeoutMs = 10000) {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    return fetch(url, { ...options, signal: controller.signal }).finally(() => clearTimeout(timer));
  }

  async function fetchFromBackendProxy() {
    const response = await fetchWithTimeout(BACKEND_UPDATE_API, {}, 12000);
    if (!response.ok) throw new Error(`backend proxy returned ${response.status}`);
    const data = await response.json();
    if (data.error) throw new Error(data.error);
    return {
      tagName: String(data?.tagName || data?.tag_name || "").trim(),
      htmlUrl: String(data?.htmlUrl || data?.html_url || "").trim(),
      body: String(data?.body || ""),
      publishedAt: String(data?.publishedAt || data?.published_at || ""),
      prerelease: Boolean(data?.prerelease)
    };
  }

  async function fetchFromGitHubDirect(url) {
    const response = await fetchWithTimeout(url, {
      headers: { Accept: "application/vnd.github+json" }
    }, 10000);
    if (!response.ok) throw new Error(`GitHub API returned ${response.status}`);
    const data = await response.json();
    return {
      tagName: String(data?.tag_name || "").trim(),
      htmlUrl: String(data?.html_url || "").trim(),
      body: String(data?.body || ""),
      publishedAt: String(data?.published_at || ""),
      prerelease: Boolean(data?.prerelease)
    };
  }

  async function fetchLatestReleaseFromGitHub() {
    // Strategy 1: Backend proxy (best option — server-side, supports system proxy)
    try {
      return await fetchFromBackendProxy();
    } catch { /* fall through */ }

    // Strategy 2: Direct GitHub API mirrors (fallback)
    for (const url of GITHUB_FALLBACK_URLS) {
      try {
        return await fetchFromGitHubDirect(url);
      } catch { /* try next mirror */ }
    }

    throw new Error("all update check sources failed");
  }

  function hideUpdateBanner() {
    if (!updateBannerHost) return;
    updateBannerHost.innerHTML = "";
    updateBannerHost.classList.add("hidden");
  }

  function renderUpdateBanner(release) {
    if (!updateBannerHost) return;

    updateBannerHost.innerHTML = "";

    const banner = document.createElement("section");
    banner.className = "update-banner";
    banner.setAttribute("role", "status");

    const title = document.createElement("div");
    title.className = "update-banner-title";
    title.innerHTML = `
      ${iconMarkup("bell-ring", "update-banner-icon", true)}
      <span>新版本可用：${escapeHtml(release.tagName)}（${escapeHtml(formatReleaseDate(release.publishedAt))}）</span>
    `;

    const summary = document.createElement("div");
    summary.className = "update-banner-summary";
    summary.textContent = `更新内容：${summarizeReleaseBody(release.body)}`;

    const actions = document.createElement("div");
    actions.className = "update-banner-actions";

    const downloadLink = document.createElement("a");
    downloadLink.className = "update-action-link";
    downloadLink.href = release.htmlUrl || "https://github.com/skxxxkx666/Kugo-Music-Converter/releases";
    downloadLink.target = "_blank";
    downloadLink.rel = "noopener noreferrer";
    downloadLink.setAttribute("aria-label", "前往下载最新版本");
    downloadLink.innerHTML = `${iconMarkup("download", "ui-icon", true)}<span>前往下载</span>`;

    const detailLink = document.createElement("a");
    detailLink.className = "update-action-link secondary";
    detailLink.href = release.htmlUrl || "https://github.com/skxxxkx666/Kugo-Music-Converter/releases";
    detailLink.target = "_blank";
    detailLink.rel = "noopener noreferrer";
    detailLink.setAttribute("aria-label", "查看更新详情");
    detailLink.innerHTML = `${iconMarkup("info", "ui-icon", true)}<span>查看详情</span>`;

    const ignoreBtn = document.createElement("button");
    ignoreBtn.type = "button";
    ignoreBtn.className = "update-action-btn";
    ignoreBtn.setAttribute("aria-label", "忽略此版本更新提示");
    ignoreBtn.innerHTML = `${iconMarkup("bell-off", "ui-icon", true)}<span>忽略此版本</span>`;
    ignoreBtn.addEventListener("click", () => {
      localStorage.setItem(UPDATE_IGNORE_KEY, release.tagName);
      hideUpdateBanner();
    });

    const closeBtn = document.createElement("button");
    closeBtn.type = "button";
    closeBtn.className = "update-action-close";
    closeBtn.setAttribute("aria-label", "关闭更新提示");
    closeBtn.innerHTML = iconMarkup("x", "ui-icon", true);
    closeBtn.addEventListener("click", hideUpdateBanner);

    actions.appendChild(downloadLink);
    actions.appendChild(detailLink);
    actions.appendChild(ignoreBtn);
    actions.appendChild(closeBtn);

    banner.appendChild(title);
    banner.appendChild(summary);
    banner.appendChild(actions);

    updateBannerHost.appendChild(banner);
    updateBannerHost.classList.remove("hidden");
    refreshIcons();
    if (hasGSAP() && !prefersReducedMotion()) {
      const iconEl = banner.querySelector(".update-banner-icon");
      const timeline = window.gsap.timeline();
      timeline.from(banner, {
        y: -60,
        opacity: 0,
        duration: 0.5,
        ease: "back.out(1.4)"
      });
      if (iconEl) {
        timeline.from(
          iconEl,
          {
            scale: 0,
            rotation: -180,
            duration: 0.4,
            ease: "back.out(2)"
          },
          "-=0.25"
        );
      }
    } else {
      slideDown(banner);
    }
  }

  async function checkForUpdates() {
    try {
      const cache = readUpdateCache();
      let release = cache?.data || null;
      const cacheFresh =
        Number.isFinite(Number(cache?.checkedAt)) && Date.now() - Number(cache.checkedAt) < UPDATE_CHECK_INTERVAL_MS;

      if (!cacheFresh || !release?.tagName) {
        release = await fetchLatestReleaseFromGitHub();
        writeUpdateCache(release);
      }

      if (!release || !release.tagName) return;
      if (localStorage.getItem(UPDATE_IGNORE_KEY) === release.tagName) return;
      if (!parseVersion(appVersion) || !parseVersion(release.tagName)) return;
      if (!shouldNotifyUpdate(appVersion, release.tagName, release.prerelease)) return;

      renderUpdateBanner(release);
    } catch {
      // 静默忽略更新检测错误
    }
  }

  return {
    checkForUpdates,
    hideUpdateBanner
  };
}
