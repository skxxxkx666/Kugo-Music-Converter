export async function readSseStream(response, onEvent) {
  const reader = response.body.getReader();
  const decoder = new TextDecoder("utf-8");
  let buffer = "";

  while (true) {
    // eslint-disable-next-line no-await-in-loop
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });

    let boundaryIndex = buffer.search(/\r?\n\r?\n/);
    while (boundaryIndex !== -1) {
      const rawEvent = buffer.slice(0, boundaryIndex);
      buffer = buffer.slice(boundaryIndex).replace(/^\r?\n\r?\n/, "");
      parseSseEvent(rawEvent, onEvent);
      boundaryIndex = buffer.search(/\r?\n\r?\n/);
    }
  }

  if (buffer.trim()) parseSseEvent(buffer, onEvent);
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
  onEvent(eventName, payload);
}
