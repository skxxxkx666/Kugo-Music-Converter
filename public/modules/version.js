function toInt(value, fallback = 0) {
  const n = Number.parseInt(String(value ?? ""), 10);
  return Number.isFinite(n) ? n : fallback;
}

export function parseVersion(input) {
  const raw = String(input || "").trim();
  const match = raw.match(/^v?(\d+)\.(\d+)\.(\d+)(?:-([a-zA-Z]+)\.(\d+))?$/);
  if (!match) return null;

  return {
    major: toInt(match[1]),
    minor: toInt(match[2]),
    patch: toInt(match[3]),
    pre: match[4] ? String(match[4]).toLowerCase() : "",
    preNum: match[5] ? toInt(match[5]) : 0,
    raw
  };
}

export function isPreviewVersion(input) {
  const parsed = parseVersion(input);
  return Boolean(parsed && parsed.pre === "preview");
}

export function compareVersions(a, b) {
  const av = typeof a === "string" ? parseVersion(a) : a;
  const bv = typeof b === "string" ? parseVersion(b) : b;
  if (!av || !bv) return 0;

  if (av.major !== bv.major) return av.major > bv.major ? 1 : -1;
  if (av.minor !== bv.minor) return av.minor > bv.minor ? 1 : -1;
  if (av.patch !== bv.patch) return av.patch > bv.patch ? 1 : -1;

  const aStable = !av.pre;
  const bStable = !bv.pre;
  if (aStable && !bStable) return 1;
  if (!aStable && bStable) return -1;
  if (aStable && bStable) return 0;

  if (av.pre !== bv.pre) return av.pre > bv.pre ? 1 : -1;
  if (av.preNum !== bv.preNum) return av.preNum > bv.preNum ? 1 : -1;
  return 0;
}

export function shouldNotifyUpdate(currentVersion, remoteVersion, remoteIsPrerelease = false) {
  const current = parseVersion(currentVersion);
  const remote = parseVersion(remoteVersion);
  if (!current || !remote) return false;

  const currentIsPreview = isPreviewVersion(currentVersion);
  if (!currentIsPreview && (remoteIsPrerelease || remote.pre === "preview")) {
    return false;
  }

  return compareVersions(remote, current) > 0;
}
