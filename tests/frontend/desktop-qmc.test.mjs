import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import assert from "node:assert/strict";

const root = path.resolve(import.meta.dirname, "../..");
const mainJS = fs.readFileSync(path.join(root, "backend/frontend/src/main.js"), "utf8");
const indexHTML = fs.readFileSync(path.join(root, "backend/frontend/src/index.html"), "utf8");

for (const extension of [".mflac", ".mgg"]) {
  test(`desktop frontend exposes ${extension}`, () => {
    assert.ok(mainJS.includes(`"${extension}"`));
    assert.ok(indexHTML.includes(extension));
  });
}

test("desktop privacy disclosure is versioned and explicit", () => {
  assert.match(mainJS, /kugo-desktop-disclaimer-v2/);
  assert.match(mainJS, /QQ 音乐兼容端点取钥/);
  assert.match(indexHTML, /只读权限扫描同用户 QQMusic\/qmbrowser 进程内存/);
  assert.match(indexHTML, /未公开兼容端点/);
});

test("desktop static version matches v0.6.1", () => {
  assert.match(indexHTML, /id="versionBadge"[^>]*>v0\.6\.1</);
});
