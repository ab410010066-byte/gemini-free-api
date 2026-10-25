// Gemini Cookie Sync - Background Service Worker

const DEFAULT_SERVER_URL = "http://localhost:8083";
const DEFAULT_ADMIN_TOKEN = "";

// 获取当前存储的配置
async function getConfig() {
  const cfg = await chrome.storage.local.get({
    serverUrl: DEFAULT_SERVER_URL,
    adminToken: DEFAULT_ADMIN_TOKEN,
    autoSync: false,
    lastSyncTime: 0,
    lastSyncAccount: ""
  });
  return cfg;
}

// 获取 Google 与 Gemini 相关的完整 Cookies
async function extractGeminiCookies() {
  const [googleCookies, geminiCookies] = await Promise.all([
    chrome.cookies.getAll({ domain: "google.com" }),
    chrome.cookies.getAll({ domain: "gemini.google.com" })
  ]);

  const cookieMap = new Map();

  // 合并并去重
  for (const c of [...googleCookies, ...geminiCookies]) {
    cookieMap.set(c.name, c.value);
  }

  // 关键登录凭证检查
  const sapisid = cookieMap.get("SAPISID");
  if (!sapisid) {
    throw new Error("未检测到有效登录态（缺少 SAPISID），请确认您已在 gemini.google.com 登录 Google 账号");
  }

  const cookieString = Array.from(cookieMap.entries())
    .map(([k, v]) => `${k}=${v}`)
    .join("; ");

  return {
    cookieString,
    sapisidTail: sapisid.slice(-4),
    has1psidts: cookieMap.has("__Secure-1PSIDTS"),
    cookieCount: cookieMap.size
  };
}

// 发送同步请求到本地 API
async function pushToLocalAPI(label, note = "via Chrome Extension") {
  const cfg = await getConfig();
  const { cookieString, sapisidTail, cookieCount } = await extractGeminiCookies();

  const url = `${cfg.serverUrl.replace(/\/+$/, "")}/admin/api/cookies/sync`;

  const resp = await fetch(url, {
    method: "POST",
    headers: {
      "Authorization": `Bearer ${cfg.adminToken}`,
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      label: label || `account-${sapisidTail}`,
      cookie: cookieString,
      note: `${note} (${cookieCount} cookies)`
    })
  });

  const resJson = await resp.json().catch(() => ({}));
  if (!resp.ok) {
    throw new Error(resJson.error || `HTTP ${resp.status} ${resp.statusText}`);
  }

  const now = Date.now();
  await chrome.storage.local.set({
    lastSyncTime: now,
    lastSyncAccount: label || `account-${sapisidTail}`
  });

  return resJson;
}

// 检查本地服务连接状态
async function checkServerHealth() {
  const cfg = await getConfig();
  try {
    const url = `${cfg.serverUrl.replace(/\/+$/, "")}/admin/api/cookies`;
    const resp = await fetch(url, {
      headers: { "Authorization": `Bearer ${cfg.adminToken}` },
      cache: "no-store"
    });
    if (!resp.ok) return { ok: false, error: `HTTP ${resp.status}` };
    const data = await resp.json();
    return { ok: true, total: data.total, enabled: data.enabled };
  } catch (err) {
    return { ok: false, error: err.message || "无法连接到本地服务" };
  }
}

// 监听来自 popup 或 content script 的指令
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  (async () => {
    try {
      if (message.action === "CHECK_HEALTH") {
        const health = await checkServerHealth();
        sendResponse({ ok: true, health });
      } else if (message.action === "SYNC_COOKIES") {
        const res = await pushToLocalAPI(message.label, message.note);
        sendResponse({ ok: true, data: res });
      } else if (message.action === "GET_PREVIEW") {
        const cookies = await extractGeminiCookies();
        sendResponse({ ok: true, cookies });
      } else {
        sendResponse({ ok: false, error: "未知指令" });
      }
    } catch (err) {
      sendResponse({ ok: false, error: err.message });
    }
  })();
  return true; // 保持通道异步返回
});

// 监听标签页更新（支持自动同步）
chrome.tabs.onUpdated.addListener(async (tabId, changeInfo, tab) => {
  if (changeInfo.status !== "complete" || !tab.url) return;
  if (!tab.url.startsWith("https://gemini.google.com")) return;

  const cfg = await getConfig();
  if (!cfg.autoSync) return;

  // 避免短时间内频繁自动同步（至少间隔 10 分钟）
  if (Date.now() - cfg.lastSyncTime < 10 * 60 * 1000) return;

  try {
    await pushToLocalAPI("", "Auto-sync on page load");
    console.log("[Gemini Cookie Sync] 页面加载时已自动同步 Cookie");
  } catch (err) {
    console.warn("[Gemini Cookie Sync] 自动同步跳过或失败:", err.message);
  }
});
