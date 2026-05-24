// 统一 API client：拼 base + 自动带 token + 统一解包 {code, message, data}
const BASE = "/api/ragent";

export function getToken() {
  return localStorage.getItem("ragent_token") || "";
}
export function setToken(t) {
  localStorage.setItem("ragent_token", t);
}
export function getUser() {
  const raw = localStorage.getItem("ragent_user");
  return raw ? JSON.parse(raw) : null;
}
export function setUser(u) {
  localStorage.setItem("ragent_user", JSON.stringify(u));
}
export function clearAuth() {
  localStorage.removeItem("ragent_token");
  localStorage.removeItem("ragent_user");
}

export function requireAuth() {
  if (!getToken()) {
    location.href = "/ui/login.html";
    throw new Error("unauthenticated");
  }
}

function headers(extra = {}) {
  const h = { ...extra };
  const t = getToken();
  if (t) h["Authorization"] = "Bearer " + t;
  return h;
}

async function unwrap(res) {
  const ct = res.headers.get("Content-Type") || "";
  if (!ct.includes("application/json")) {
    throw new Error("非 JSON 响应：" + res.status);
  }
  const body = await res.json();
  if (body.code !== "0") {
    if (res.status === 401) {
      clearAuth();
      location.href = "/ui/login.html";
    }
    throw new Error(body.message || "请求失败");
  }
  return body.data;
}

export async function apiGet(path, query) {
  let url = BASE + path;
  if (query) {
    const qs = new URLSearchParams(query).toString();
    if (qs) url += "?" + qs;
  }
  const res = await fetch(url, { headers: headers() });
  return unwrap(res);
}

export async function apiPost(path, body) {
  const res = await fetch(BASE + path, {
    method: "POST",
    headers: headers({ "Content-Type": "application/json" }),
    body: JSON.stringify(body || {}),
  });
  return unwrap(res);
}

export async function apiPut(path, body) {
  const res = await fetch(BASE + path, {
    method: "PUT",
    headers: headers({ "Content-Type": "application/json" }),
    body: JSON.stringify(body || {}),
  });
  return unwrap(res);
}

export async function apiDelete(path) {
  const res = await fetch(BASE + path, {
    method: "DELETE",
    headers: headers(),
  });
  return unwrap(res);
}

export async function apiUpload(path, formData) {
  const res = await fetch(BASE + path, {
    method: "POST",
    headers: headers(), // 不要手动设 Content-Type，浏览器会带 boundary
    body: formData,
  });
  return unwrap(res);
}

// SSE：POST /chat/stream，手动解析 event/data 帧
// callbacks: { onChunk(delta), onDone({answer, chunks}), onError(msg) }
export async function chatStream(body, callbacks) {
  const res = await fetch(BASE + "/chat/stream", {
    method: "POST",
    headers: headers({ "Content-Type": "application/json" }),
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    if (res.status === 401) {
      clearAuth();
      location.href = "/ui/login.html";
    }
    callbacks.onError && callbacks.onError("HTTP " + res.status);
    return;
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buf = "";

  while (true) {
    const { value, done } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });

    // SSE 帧之间用空行分隔
    let idx;
    while ((idx = buf.indexOf("\n\n")) !== -1) {
      const frame = buf.slice(0, idx);
      buf = buf.slice(idx + 2);
      parseFrame(frame, callbacks);
    }
  }
}

function parseFrame(frame, cb) {
  let event = "message";
  const dataLines = [];
  for (const line of frame.split("\n")) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
  }
  const dataStr = dataLines.join("\n");
  if (!dataStr) return;
  let data;
  try { data = JSON.parse(dataStr); } catch { return; }

  if (event === "chunk") cb.onChunk && cb.onChunk(data.delta || "");
  else if (event === "done") cb.onDone && cb.onDone(data);
  else if (event === "error") cb.onError && cb.onError(data.message || "未知错误");
}