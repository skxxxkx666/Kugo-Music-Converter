export async function readSseStream(response, onEvent) {
  if (!response.body) {
    throw new Error("响应体为空，无法读取 SSE 流");
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8");
  let buffer = "";
  let sawComplete = false;
  let sawError = false;

  const emit = (eventName, payload) => {
    if (eventName === "complete") sawComplete = true;
    if (eventName === "error") sawError = true;
    if (typeof onEvent === "function") onEvent(eventName, payload);
  };

  try {
    while (true) {
      // eslint-disable-next-line no-await-in-loop
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      let boundaryIndex = buffer.search(/\r?\n\r?\n/);
      while (boundaryIndex !== -1) {
        const rawEvent = buffer.slice(0, boundaryIndex);
        buffer = buffer.slice(boundaryIndex).replace(/^\r?\n\r?\n/, "");
        parseSseEvent(rawEvent, emit);
        boundaryIndex = buffer.search(/\r?\n\r?\n/);
      }
    }

    if (buffer.trim()) parseSseEvent(buffer, emit);
  } finally {
    reader.releaseLock();
  }

  return {
    sawComplete,
    sawError
  };
}

export function parseSseEvent(raw, onEvent) {
  const lines = raw.split(/\r?\n/);
  let eventName = "message";
  const dataLines = [];

  for (const line of lines) {
    if (line.startsWith("event:")) eventName = line.slice(6).trim();
    else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
  }

  if (dataLines.length === 0) return;

  const dataText = dataLines.join("\n");
  let payload;
  try {
    payload = JSON.parse(dataText);
  } catch {
    payload = { raw: dataText };
  }
  if (typeof onEvent === "function") onEvent(eventName, payload);
}
