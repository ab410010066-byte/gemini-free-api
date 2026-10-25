// Gemini Cookie Sync - Popup Logic

document.addEventListener("DOMContentLoaded", async () => {
  const statusBadge = document.getElementById("statusBadge");
  const statusText = document.getElementById("statusText");
  const tabStatus = document.getElementById("tabStatus");
  const serverHost = document.getElementById("serverHost");
  const poolCount = document.getElementById("poolCount");
  const syncBtn = document.getElementById("syncBtn");
  const msgBox = document.getElementById("msgBox");
  const toggleSettings = document.getElementById("toggleSettings");
  const settingsPanel = document.getElementById("settingsPanel");
  const cfgServerUrl = document.getElementById("cfgServerUrl");
  const cfgAdminToken = document.getElementById("cfgAdminToken");
  const cfgLabel = document.getElementById("cfgLabel");
  const cfgAutoSync = document.getElementById("cfgAutoSync");
  const saveCfgBtn = document.getElementById("saveCfgBtn");
  const linkGemini = document.getElementById("linkGemini");
  const linkAdmin = document.getElementById("linkAdmin");

  // 加载当前设置
  const cfg = await chrome.storage.local.get({
    serverUrl: "http://localhost:8083",
    adminToken: "",
    autoSync: false,
    lastSyncAccount: ""
  });

  cfgServerUrl.value = cfg.serverUrl;
  cfgAdminToken.value = cfg.adminToken;
  cfgAutoSync.checked = cfg.autoSync;
  serverHost.textContent = cfg.serverUrl;

  // 检查当前标签页
  const [activeTab] = await chrome.tabs.query({ active: true, currentWindow: true });
  const isGeminiPage = activeTab && activeTab.url && activeTab.url.startsWith("https://gemini.google.com");
  if (isGeminiPage) {
    tabStatus.textContent = "🟢 在 Gemini 頁面";
    tabStatus.style.color = "var(--success)";
  } else {
    tabStatus.textContent = "⚪ 未在 Gemini 頁面";
    tabStatus.style.color = "var(--text-muted)";
  }

  // 检查本地服务器状态
  async function refreshServerStatus() {
    try {
      const resp = await chrome.runtime.sendMessage({ action: "CHECK_HEALTH" });
      if (resp && resp.ok && resp.health.ok) {
        statusBadge.className = "status-badge";
        statusText.textContent = "已連線";
        poolCount.textContent = `${resp.health.enabled}/${resp.health.total} 帳號就緒`;
      } else {
        statusBadge.className = "status-badge error";
        statusText.textContent = "未連線";
        poolCount.textContent = "無法連線至 API";
      }
    } catch (err) {
      statusBadge.className = "status-badge error";
      statusText.textContent = "未連線";
      poolCount.textContent = "連線失敗";
    }
  }

  await refreshServerStatus();

  // 点击同步
  syncBtn.addEventListener("click", async () => {
    syncBtn.disabled = true;
    syncBtn.innerHTML = `<span>⏳</span> 正在抓取並同步...`;
    msgBox.className = "msg-box";
    msgBox.style.display = "none";

    try {
      const label = cfgLabel.value.trim() || cfg.lastSyncAccount;
      const resp = await chrome.runtime.sendMessage({
        action: "SYNC_COOKIES",
        label: label,
        note: "via Extension Popup"
      });

      if (resp && resp.ok) {
        const actionStr = resp.data.action === "updated" ? "已更新既有帳號" : "已成功新增帳號";
        const displayLabel = resp.data.label || `#${resp.data.id}`;
        msgBox.className = "msg-box success";
        msgBox.innerHTML = `🎉 <b>${actionStr}</b>：${displayLabel}<br>已即時生效於後台！`;
        await refreshServerStatus();
      } else {
        throw new Error(resp ? resp.error : "同步失敗");
      }
    } catch (err) {
      msgBox.className = "msg-box error";
      msgBox.innerHTML = `❌ <b>同步出錯</b>：${err.message}`;
    } finally {
      syncBtn.disabled = false;
      syncBtn.innerHTML = `<span>🚀</span> 一鍵抓取並同步此帳號`;
    }
  });

  // 折叠设置
  toggleSettings.addEventListener("click", () => {
    settingsPanel.classList.toggle("open");
  });

  // 保存设置
  saveCfgBtn.addEventListener("click", async () => {
    await chrome.storage.local.set({
      serverUrl: cfgServerUrl.value.trim().replace(/\/+$/, ""),
      adminToken: cfgAdminToken.value.trim(),
      autoSync: cfgAutoSync.checked
    });
    serverHost.textContent = cfgServerUrl.value;
    saveCfgBtn.textContent = "已保存！";
    setTimeout(() => { saveCfgBtn.textContent = "保存設置"; }, 1500);
    await refreshServerStatus();
  });

  // 快捷链接
  linkGemini.addEventListener("click", (e) => {
    e.preventDefault();
    chrome.tabs.create({ url: "https://gemini.google.com" });
  });

  linkAdmin.addEventListener("click", (e) => {
    e.preventDefault();
    const adminUrl = `${cfgServerUrl.value.trim().replace(/\/+$/, "")}/admin/?token=${encodeURIComponent(cfgAdminToken.value.trim())}`;
    chrome.tabs.create({ url: adminUrl });
  });
});
