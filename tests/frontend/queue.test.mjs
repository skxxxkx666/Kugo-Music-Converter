import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

async function importSource(relativePath) {
  const sourceUrl = new URL(relativePath, import.meta.url);
  const source = await readFile(sourceUrl, "utf8");
  const encoded = Buffer.from(source).toString("base64");
  return import(`data:text/javascript;base64,${encoded}#${encodeURIComponent(relativePath)}`);
}

function fakeFile(name, size, lastModified = 1) {
  return { name, size, lastModified };
}

test("mergeFiles returns accurate accepted and skipped counts", async () => {
  const { createUploaderController } = await importSource("../../public/modules/uploader.js");
  const logs = [];
  const state = {
    maxFileCount: 3,
    maxFileSizeMB: 1024,
    selectedFiles: [],
    pathQueue: [],
    supportedFormats: [".kgg", ".kgm", ".kgma", ".vpr", ".ncm"],
  };
  const uploader = createUploaderController({
    state,
    elements: {},
    helpers: {
      getExt: (name) => `.${String(name).split(".").pop().toLowerCase()}`,
      formatBytes: (bytes) => `${bytes} B`,
      renderExtBadge: String,
      escapeHtml: String,
      setHintStatus: () => {},
      appendLog: (level, message) => logs.push(message),
      refreshIcons: () => {},
    },
  });
  const first = fakeFile("first.kgm", 250 * 1024 * 1024);

  const stats = uploader.mergeFiles([
    first,
    fakeFile("first.kgm", first.size),
    fakeFile("notes.txt", 10),
    fakeFile("huge.kgm", 1024 * 1024 * 1024 + 1),
    fakeFile("second.kgg", 20),
    fakeFile("third.ncm", 30),
    fakeFile("fourth.vpr", 40),
  ]);

  assert.deepEqual(stats, {
    candidates: 7,
    added: 3,
    unsupported: 1,
    tooLarge: 1,
    duplicate: 1,
    blockedByLimit: 1,
  });
  assert.equal(state.selectedFiles.length, 3);
  assert.ok(logs.some((message) => message.includes("huge.kgm")));
});

test("scan queue fills every remaining slot", async () => {
  const { createScanner } = await importSource("../../public/modules/scanner.js");
  const state = {
    maxFileCount: 3,
    pathQueue: [{ fullPath: "C:\\music\\existing.kgg", name: "existing.kgg", ext: ".kgg" }],
    scanFiles: [
      { fullPath: "C:\\music\\second.kgm", name: "second.kgm", ext: ".kgm", size: 20 },
      { fullPath: "C:\\music\\third.ncm", name: "third.ncm", ext: ".ncm", size: 30 },
    ],
  };
  const scanner = createScanner({
    state,
    elements: {},
    appendLog: () => {},
    pendingCount: () => state.pathQueue.length,
    onQueueChanged: () => {},
  });

  scanner.addScanFilesToQueue();

  assert.equal(state.pathQueue.length, 3);
  assert.deepEqual(
    state.pathQueue.map((item) => item.name),
    ["existing.kgg", "second.kgm", "third.ncm"],
  );
});

test("upload total size is calculated from valid numeric file sizes", async () => {
  const { getUploadTotalBytes } = await importSource("../../public/modules/utils.js");

  assert.equal(
    getUploadTotalBytes([
      fakeFile("one.kgm", 250 * 1024 * 1024),
      fakeFile("two.kgg", 500 * 1024 * 1024),
      { name: "invalid.ncm", size: Number.NaN },
    ]),
    750 * 1024 * 1024,
  );
});
