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

test("desktop queue advertises QQ Music MFLAC and MGG support", () => {
  assert.match(indexHTML, /酷我、QQ 音乐加密音频/);
  assert.match(indexHTML, /<span>MFLAC<\/span>/);
  assert.match(indexHTML, /<span>MGG<\/span>/);
});

test("desktop privacy disclosure is versioned and explicit", () => {
  assert.match(mainJS, /kugo-desktop-disclaimer-v2/);
  assert.match(mainJS, /仅新版 QQ 音乐 MFLAC\/MGG 需要在登录客户端后联网获取解密密钥/);
  assert.match(indexHTML, /音频文件不会发送到网络/);
  assert.match(indexHTML, /QQ 音乐服务端兼容接口/);
  assert.match(indexHTML, /不会上传音频内容或本地文件路径/);
});

test("desktop static version matches v0.6.1", () => {
  assert.match(indexHTML, /id="versionBadge"[^>]*>v0\.6\.1</);
});
