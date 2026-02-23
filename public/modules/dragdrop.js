function readFileFromEntry(entry) {
  return new Promise((resolve) => {
    entry.file(
      (file) => resolve(file),
      () => resolve(null)
    );
  });
}

function readDirectoryEntries(reader) {
  return new Promise((resolve) => {
    const all = [];
    const readBatch = () => {
      reader.readEntries(
        (entries) => {
          if (!entries || entries.length === 0) {
            resolve(all);
            return;
          }
          all.push(...entries);
          readBatch();
        },
        () => resolve(all)
      );
    };
    readBatch();
  });
}

async function collectFilesFromEntry(entry, sink, depth = 0) {
  if (!entry || depth > 64) return;
  if (entry.isFile) {
    const file = await readFileFromEntry(entry);
    if (file) sink.push(file);
    return;
  }
  if (!entry.isDirectory) return;

  const reader = entry.createReader();
  const entries = await readDirectoryEntries(reader);
  await Promise.all(entries.map((child) => collectFilesFromEntry(child, sink, depth + 1)));
}

async function extractDroppedFiles(dataTransfer) {
  const fallbackFiles = Array.from(dataTransfer?.files || []);
  const items = Array.from(dataTransfer?.items || []);
  const supportsEntryApi = items.some((item) => typeof item?.webkitGetAsEntry === "function");
  if (!supportsEntryApi) return fallbackFiles;

  const collected = [];
  for (const item of items) {
    const entry = item.webkitGetAsEntry?.();
    if (!entry) {
      const file = item.getAsFile?.();
      if (file) collected.push(file);
      continue;
    }
    // 非标准 API：目录拖放递归解析（Chrome/Edge/Firefox 兼容）。
    await collectFilesFromEntry(entry, collected);
  }

  return collected.length > 0 ? collected : fallbackFiles;
}

export function createDragDropController(options) {
  const {
    dropZone,
    isFileDragEvent,
    hasGSAP,
    prefersReducedMotion,
    pulseDropZone,
    onFilesDropped
  } = options;

  let windowDragDepth = 0;
  let dropZoneDragDepth = 0;
  let bound = false;

  function resetDraggingState() {
    windowDragDepth = 0;
    dropZoneDragDepth = 0;
    document.body.classList.remove("dragging-files");
    dropZone.classList.remove("drag-over");
  }

  function bind() {
    if (bound) return;
    bound = true;

    window.addEventListener("dragenter", (event) => {
      if (!isFileDragEvent(event)) return;
      windowDragDepth += 1;
      document.body.classList.add("dragging-files");
    });
    window.addEventListener("dragleave", (event) => {
      if (!isFileDragEvent(event)) return;
      windowDragDepth = Math.max(0, windowDragDepth - 1);
      if (windowDragDepth === 0) document.body.classList.remove("dragging-files");
    });
    window.addEventListener("drop", resetDraggingState);
    window.addEventListener("dragend", resetDraggingState);
    window.addEventListener("blur", resetDraggingState);

    dropZone.addEventListener("dragenter", (event) => {
      event.preventDefault();
      if (!isFileDragEvent(event)) return;
      dropZoneDragDepth += 1;
      dropZone.classList.add("drag-over");
      if (typeof pulseDropZone === "function") pulseDropZone();
    });

    dropZone.addEventListener("dragover", (event) => {
      event.preventDefault();
      if (!isFileDragEvent(event)) return;
      if (!dropZone.classList.contains("drag-over")) {
        dropZone.classList.add("drag-over");
        if (typeof pulseDropZone === "function") pulseDropZone();
      }
    });

    dropZone.addEventListener("dragleave", (event) => {
      if (!isFileDragEvent(event)) return;
      dropZoneDragDepth = Math.max(0, dropZoneDragDepth - 1);
      if (dropZoneDragDepth === 0) dropZone.classList.remove("drag-over");
    });

    dropZone.addEventListener("drop", async (event) => {
      event.preventDefault();
      if (!isFileDragEvent(event)) return;
      resetDraggingState();

      if (hasGSAP() && !prefersReducedMotion()) {
        window.gsap.to(dropZone, { scale: 1, duration: 0.2, ease: "power1.out" });
      }

      const droppedFiles = await extractDroppedFiles(event.dataTransfer);
      if (typeof onFilesDropped === "function") {
        await onFilesDropped(droppedFiles);
      }
    });
  }

  return {
    bind,
    resetDraggingState,
    extractDroppedFiles
  };
}
