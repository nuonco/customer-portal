// dev-reload.js — Idiomorph-based hot reload for local development.
// Polls /dev/version; when the server restarts with new code, morphs
// the page in-place instead of doing a full reload.
(function () {
  var POLL_MS = 1500;
  var FAIL_THRESHOLD = 4; // after ~6s unreachable, assume build error
  var currentVersion = null;
  var overlay = null;
  var failCount = 0;

  // --- Overlay UI ---

  function createOverlay() {
    overlay = document.createElement("div");
    overlay.id = "dev-reload-overlay";
    overlay.style.cssText =
      "position:fixed;bottom:20px;right:20px;z-index:99999;" +
      "background:#1a1a2e;color:#e0e0e0;padding:12px 20px;border-radius:8px;" +
      "font:14px/1.4 Inter,system-ui,sans-serif;box-shadow:0 4px 12px rgba(0,0,0,.3);" +
      "transition:background .3s ease;max-width:600px;";
    var style = document.createElement("style");
    style.textContent = "@keyframes dev-spin{to{transform:rotate(360deg)}}";
    overlay.appendChild(style);
    overlay.innerHTML +=
      '<div style="display:flex;align-items:center;gap:12px">' +
      '<span class="dev-reload-icon"></span>' +
      '<span class="dev-reload-label"></span></div>';
    document.body.appendChild(overlay);
  }

  var spinnerSVG =
    '<svg width="20" height="20" viewBox="0 0 24 24" style="animation:dev-spin 1s linear infinite">' +
    '<circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" fill="none" stroke-dasharray="31 31"/></svg>';

  var errorSVG =
    '<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#ff6b6b" stroke-width="2">' +
    '<circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>';

  function showRebuilding() {
    if (!overlay) createOverlay();
    overlay.style.background = "#1a1a2e";
    overlay.querySelector(".dev-reload-icon").innerHTML = spinnerSVG;
    overlay.querySelector(".dev-reload-label").textContent = "Rebuilding\u2026";
  }

  function showBuildError() {
    if (!overlay) createOverlay();
    overlay.style.background = "#4a1020";
    overlay.querySelector(".dev-reload-icon").innerHTML = errorSVG;
    var label = overlay.querySelector(".dev-reload-label");
    label.innerHTML = 'Build failed \u2014 <a href="http://localhost:7777/logs.html?service=customer-dashboard" target="_blank" style="color:#ff6b6b;text-decoration:underline">view logs</a>';
  }

  function hideOverlay() {
    if (!overlay) return;
    overlay.remove();
    overlay = null;
    failCount = 0;
  }

  // --- Head diffing ---

  function diffHead(newDoc) {
    var oldLinks = {};
    document.querySelectorAll('head link[rel="stylesheet"]').forEach(function (l) {
      oldLinks[l.getAttribute("href")] = l;
    });
    var newLinks = {};
    newDoc.querySelectorAll('head link[rel="stylesheet"]').forEach(function (l) {
      newLinks[l.getAttribute("href")] = l;
    });
    Object.keys(oldLinks).forEach(function (href) {
      if (!newLinks[href]) oldLinks[href].remove();
    });
    Object.keys(newLinks).forEach(function (href) {
      if (!oldLinks[href]) document.head.appendChild(newLinks[href].cloneNode(true));
    });
    var oldStyles = Array.from(document.querySelectorAll("head style"));
    var newStyles = Array.from(newDoc.querySelectorAll("head style"));
    for (var i = 0; i < Math.max(oldStyles.length, newStyles.length); i++) {
      if (i < oldStyles.length && i < newStyles.length) {
        if (oldStyles[i].textContent !== newStyles[i].textContent) {
          oldStyles[i].textContent = newStyles[i].textContent;
        }
      } else if (i >= oldStyles.length) {
        document.head.appendChild(newStyles[i].cloneNode(true));
      }
    }
  }

  // --- Page morph ---

  async function morphPage() {
    try {
      var resp = await fetch(location.href, {
        headers: { Accept: "text/html" },
        cache: "no-store",
      });
      if (!resp.ok) {
        console.log("[dev-reload] page fetch failed:", resp.status);
        return;
      }
      var html = await resp.text();
      var parser = new DOMParser();
      var newDoc = parser.parseFromString(html, "text/html");
      if (typeof Idiomorph !== "undefined") {
        console.log("[dev-reload] morphing body...");
        Idiomorph.morph(document.body, newDoc.body, {
          morphStyle: "innerHTML",
          ignoreActiveValue: true,
        });
      } else {
        console.warn("[dev-reload] Idiomorph not available, falling back to reload");
        location.reload();
        return;
      }
      diffHead(newDoc);
    } catch (e) {
      console.error("[dev-reload] morphPage error:", e);
    }
  }

  // --- Poll loop ---

  async function poll() {
    try {
      var resp = await fetch("/dev/version", { cache: "no-store" });
      var version = await resp.text();

      if (currentVersion === null) {
        console.log("[dev-reload] connected, version:", version);
        currentVersion = version;
        hideOverlay();
      } else if (version !== currentVersion) {
        console.log("[dev-reload] version changed:", currentVersion, "→", version);
        currentVersion = version;
        await morphPage();
        console.log("[dev-reload] morph complete");
        hideOverlay();
      } else {
        hideOverlay();
      }
    } catch (e) {
      failCount++;
      if (failCount >= FAIL_THRESHOLD) {
        showBuildError();
      } else {
        showRebuilding();
      }
    }
    setTimeout(poll, POLL_MS);
  }

  if (document.readyState === "complete") {
    poll();
  } else {
    window.addEventListener("load", poll);
  }
})();
