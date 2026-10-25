// Gemini Cookie Sync - Content Script (Injected on gemini.google.com)

(function () {
  if (window.__gemini_cookie_sync_injected) return;
  window.__gemini_cookie_sync_injected = true;

  // 尝试从网页 DOM 提取当前登录账号的 Email 或名称
  function detectAccountIdentifier() {
    // 1. Google 账号头像按钮的 aria-label: e.g. "Google 帳戶: Example User (user@example.com)"
    const accountButtons = document.querySelectorAll(
      'a[aria-label*="@"], button[aria-label*="@"], a[href*="accounts.google.com"], [data-email]'
    );
    for (const btn of accountButtons) {
      const emailAttr = btn.getAttribute("data-email");
      if (emailAttr && emailAttr.includes("@")) return emailAttr;

      const aria = btn.getAttribute("aria-label") || "";
      const match = aria.match(/([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})/);
      if (match) return match[1];
    }

    // 2. 检查页面上的图片或 alt 属性
    const avatar = document.querySelector('img[alt*="@"], img[aria-label*="@"]');
    if (avatar) {
      const text = avatar.alt || avatar.getAttribute("aria-label") || "";
      const match = text.match(/([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})/);
      if (match) return match[1];
    }

    // 3. 页面全局文本中的 email 格式探测
    const bodyText = document.body ? document.body.innerText.slice(0, 10000) : "";
    const emailMatch = bodyText.match(/[a-zA-Z0-9._%+-]+@gmail\.com/);
    if (emailMatch) return emailMatch[0];

    return "";
  }

  // 创建并注入右下角浮动同步胶囊
  function injectFloatingButton() {
    if (document.getElementById("gemini-cookie-sync-host")) return;

    const host = document.createElement("div");
    host.id = "gemini-cookie-sync-host";

    const btn = document.createElement("div");
    btn.className = "gcs-pill";
    btn.innerHTML = `
      <span class="gcs-icon">⚡</span>
      <span class="gcs-text">同步此帳號到 API</span>
    `;

    host.appendChild(btn);
    document.body.appendChild(host);

    btn.addEventListener("click", async () => {
      if (btn.classList.contains("syncing")) return;

      const email = detectAccountIdentifier();
      btn.className = "gcs-pill syncing";
      btn.innerHTML = `<span class="gcs-icon gcs-spin">⏳</span><span class="gcs-text">正在同步...</span>`;

      try {
        const resp = await chrome.runtime.sendMessage({
          action: "SYNC_COOKIES",
          label: email,
          note: "via Gemini Web Float Button"
        });

        if (resp && resp.ok) {
          const actionText = resp.data.action === "updated" ? "已更新" : "已新增";
          const displayLabel = email || resp.data.label || `#${resp.data.id}`;
          btn.className = "gcs-pill success";
          btn.innerHTML = `<span class="gcs-icon">✅</span><span class="gcs-text">${actionText}: ${displayLabel}</span>`;
        } else {
          throw new Error(resp ? resp.error : "未知错误");
        }
      } catch (err) {
        btn.className = "gcs-pill error";
        btn.innerHTML = `<span class="gcs-icon">❌</span><span class="gcs-text">${err.message.slice(0, 24)}</span>`;
      }

      setTimeout(() => {
        btn.className = "gcs-pill";
        btn.innerHTML = `<span class="gcs-icon">⚡</span><span class="gcs-text">同步此帳號到 API</span>`;
      }, 3500);
    });
  }

  // 页面加载完成后延迟 1.5 秒注入（避开 SPA 初次路由加载抖动）
  if (document.readyState === "complete" || document.readyState === "interactive") {
    setTimeout(injectFloatingButton, 1500);
  } else {
    window.addEventListener("DOMContentLoaded", () => setTimeout(injectFloatingButton, 1500));
  }
})();
