// dev-reload.js — Idiomorph-based hot reload for local development.
// Polls /dev/version; when the server restarts with new code, morphs
// the page in-place instead of doing a full reload.
(function () {
  var POLL_MS = 1500;
  var NUONCTL_STATUS_URL = "http://localhost:7777/api/status/poll";
  var NUONCTL_FAIL_THRESHOLD = 10; // fallback if nuonctl itself is unreachable (~15s)
  var currentVersion = null;
  var indicator = null;
  var failCount = 0;
  var currentState = "idle"; // idle | building | error

  // --- Nuon logo SVG (white, sized to fit the pill) ---
  var nuonSVG =
    '<svg width="32" height="32" viewBox="0 0 201 201" fill="none" xmlns="http://www.w3.org/2000/svg">' +
    '<path d="M121.15 40.3118L97.9645 53.715V75.4151L79.1959 64.5597H79.1852L56.8232 77.492V148.651L79.1852 161.584H79.1959L103.205 147.699V126.951L121.161 137.325L144.346 123.922V53.715L121.161 40.3118H121.15ZM62.0528 80.5216L79.1745 70.6297H79.1852L97.9538 81.4744V117.862L62.0528 97.1151V80.5216ZM97.9538 144.669L79.1745 155.514L62.0528 145.622V103.174L97.9538 123.922V144.669ZM139.095 120.881L121.15 131.255L103.205 120.892V84.504L139.106 105.251V120.881H139.095ZM139.095 99.192L103.194 78.4447V56.7447L121.15 46.3711L139.095 56.7447V99.192Z" fill="white"/>' +
    '</svg>';

  var spinnerSVG =
    '<svg width="24" height="24" viewBox="0 0 24 24" style="animation:dev-spin 1s linear infinite">' +
    '<circle cx="12" cy="12" r="10" stroke="white" stroke-width="3" fill="none" stroke-dasharray="31 31"/></svg>';

  var errorSVG =
    '<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#ff6b6b" stroke-width="2">' +
    '<circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>';

  // --- Indicator UI ---

  var menu = null;
  var menuOpen = false;

  function getInstallID() {
    var m = window.location.pathname.match(/\/installs\/([^/]+)/);
    return m ? m[1] : null;
  }

  function toggleMenu() {
    if (menuOpen) { closeMenu(); return; }
    if (!menu) {
      menu = document.createElement("div");
      menu.id = "dev-reload-menu";
      document.body.appendChild(menu);
    }
    var items = [
      '<a class="dev-menu-item" href="http://localhost:7777/logs.html?service=customer-dashboard" target="_blank">' +
        '<i class="ph-bold ph-terminal-window"></i> nuonctl logs</a>'
    ];
    var installID = getInstallID();
    if (installID) {
      items.push(
        '<a class="dev-menu-item" href="/installs/' + installID + '/debug">' +
          '<i class="ph-bold ph-bug"></i> Debug</a>'
      );
    }
    menu.innerHTML = items.join("");
    menu.classList.add("open");
    menuOpen = true;
    setTimeout(function () {
      document.addEventListener("click", outsideClickHandler);
    }, 0);
  }

  function closeMenu() {
    if (menu) menu.classList.remove("open");
    menuOpen = false;
    document.removeEventListener("click", outsideClickHandler);
  }

  function outsideClickHandler(e) {
    if (indicator && indicator.contains(e.target)) return;
    if (menu && menu.contains(e.target)) return;
    closeMenu();
  }

  function createIndicator() {
    var style = document.createElement("style");
    style.textContent =
      "@keyframes dev-spin{to{transform:rotate(360deg)}}" +
      "#dev-reload-indicator{" +
        "position:fixed;bottom:20px;right:20px;z-index:99999;" +
        "display:flex;align-items:center;gap:10px;flex-wrap:wrap;" +
        "background:rgba(17,17,17,0.5);color:#e0e0e0;border-radius:999px;" +
        "font:13px/1.4 Inter,system-ui,sans-serif;" +
        "box-shadow:0 2px 8px rgba(0,0,0,.4);" +
        "transition:all .3s ease;cursor:pointer;" +
        "padding:10px;" +
      "}" +
      "#dev-reload-indicator.state-error{" +
        "padding:10px 16px;border-radius:999px;" +
        "background:rgba(74,16,32,0.5);" +
      "}" +
      "#dev-reload-indicator .dev-icon{" +
        "flex-shrink:0;width:24px;height:24px;display:flex;align-items:center;justify-content:center;" +
      "}" +
      "#dev-reload-indicator .dev-detail{" +
        "display:none;white-space:nowrap;" +
      "}" +
      "#dev-reload-indicator.state-error .dev-detail{display:inline;}" +
      "#dev-reload-indicator .dev-error-msg{" +
        "display:none;margin-top:6px;padding:6px 8px;" +
        "background:rgba(0,0,0,.3);border-radius:4px;" +
        "font-size:12px;max-height:200px;overflow:auto;" +
        "white-space:pre-wrap;color:#ff9999;" +
      "}" +
      "#dev-reload-indicator.state-error .dev-error-msg.has-content{display:block;}" +
      "#dev-reload-indicator:hover{background:rgba(17,17,17,0.9);}" +
      "#dev-reload-indicator.state-error:hover{background:rgba(74,16,32,0.9);}" +
      "#dev-reload-menu{" +
        "position:fixed;bottom:60px;right:20px;z-index:99999;" +
        "background:rgba(17,17,17,0.95);border-radius:8px;" +
        "box-shadow:0 4px 16px rgba(0,0,0,.5);" +
        "font:13px/1.4 Inter,system-ui,sans-serif;" +
        "padding:4px 0;display:none;min-width:160px;" +
      "}" +
      "#dev-reload-menu.open{display:block;}" +
      ".dev-menu-item{" +
        "display:flex;align-items:center;gap:8px;" +
        "padding:8px 14px;color:#e0e0e0;text-decoration:none;" +
        "white-space:nowrap;" +
      "}" +
      ".dev-menu-item:hover{background:rgba(255,255,255,0.1);}";
    document.head.appendChild(style);

    indicator = document.createElement("div");
    indicator.id = "dev-reload-indicator";
    indicator.className = "state-idle";
    indicator.innerHTML =
      '<span class="dev-icon"></span>' +
      '<span class="dev-detail"></span>' +
      '<pre class="dev-error-msg"></pre>';
    indicator.addEventListener("click", function (e) {
      e.stopPropagation();
      toggleMenu();
    });
    document.body.appendChild(indicator);
    setIdle();
  }

  function setIdle() {
    if (!indicator) createIndicator();
    currentState = "idle";
    indicator.className = "state-idle";
    indicator.querySelector(".dev-icon").innerHTML = nuonSVG;
    indicator.querySelector(".dev-detail").textContent = "";
    var pre = indicator.querySelector(".dev-error-msg");
    pre.textContent = "";
    pre.classList.remove("has-content");
  }

  function setBuilding() {
    if (!indicator) createIndicator();
    if (currentState === "building") return;
    currentState = "building";
    indicator.className = "state-building";
    indicator.querySelector(".dev-icon").innerHTML = spinnerSVG;
    indicator.querySelector(".dev-detail").textContent = "";
    var pre = indicator.querySelector(".dev-error-msg");
    pre.textContent = "";
    pre.classList.remove("has-content");
  }

  function setError(errorMsg) {
    if (!indicator) createIndicator();
    currentState = "error";
    indicator.className = "state-error";
    indicator.querySelector(".dev-icon").innerHTML = errorSVG;
    indicator.querySelector(".dev-detail").innerHTML =
      'Build failed \u2014 <a href="http://localhost:7777/logs.html?service=customer-dashboard" ' +
      'target="_blank" style="color:#ff6b6b;text-decoration:underline">view logs</a>';
    var pre = indicator.querySelector(".dev-error-msg");
    if (errorMsg) {
      pre.textContent = errorMsg;
      pre.classList.add("has-content");
    } else {
      pre.textContent = "";
      pre.classList.remove("has-content");
    }
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
      // Re-append indicator and menu — the morph replaced body innerHTML
      if (indicator && !document.body.contains(indicator)) {
        document.body.appendChild(indicator);
      }
      if (menu && !document.body.contains(menu)) {
        document.body.appendChild(menu);
      }
      // After morphing, HTMX needs to re-process new elements so that
      // hx-trigger="load" fires again for freshly inserted content.
      if (typeof htmx !== "undefined") {
        htmx.process(document.body);
      }
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
        setIdle();
      } else if (version !== currentVersion) {
        console.log("[dev-reload] version changed:", currentVersion, "→", version);
        currentVersion = version;
        await morphPage();
        console.log("[dev-reload] morph complete");
        setIdle();
      } else {
        setIdle();
      }
      failCount = 0;
    } catch (e) {
      failCount++;
      // Ask nuonctl for the real build status instead of guessing from a timeout.
      try {
        var statusResp = await fetch(NUONCTL_STATUS_URL, { cache: "no-store" });
        var statusData = await statusResp.json();
        var svc = (statusData.services || []).find(function (s) {
          return s.name === "customer-dashboard";
        });
        if (svc) {
          if (svc.status === "Failed" || svc.status === "CrashLoop") {
            setError(svc.lastError || null);
          } else {
            setBuilding();
          }
        } else {
          setBuilding();
        }
      } catch (_) {
        // nuonctl itself is unreachable — fall back to threshold
        if (failCount >= NUONCTL_FAIL_THRESHOLD) {
          setError();
        } else {
          setBuilding();
        }
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
