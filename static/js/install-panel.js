// install-panel.js — Installs page panel logic.
// Uses event delegation so it survives Idiomorph DOM morphing.
(function () {
  var currentInstallId = null;
  var historyLoaded = false;
  var auditLoaded = false;

  function basePath() {
    var el = document.getElementById("installs-page-config");
    return el ? el.dataset.basePath : "";
  }

  function preparePanel(row, installId, installName) {
    currentInstallId = installId;
    historyLoaded = false;
    auditLoaded = false;

    document.getElementById("panel-install-name").textContent = installName;
    document.getElementById("panel-install-id").textContent = installId;
    document.getElementById("panel-actions-container").innerHTML = "";

    document.querySelectorAll(".install-row").forEach(function (r) {
      r.classList.remove("active");
    });
    row.classList.add("active");

    document.getElementById("detail-panel-overlay").classList.add("open");
    document.getElementById("detail-panel").classList.add("open");
    document.body.style.overflow = "hidden";

    switchTab("overview");

    document.getElementById("panel-content-overview").innerHTML =
      '<div class="panel-loading"><div class="panel-loading-spinner"></div></div>';
  }

  function finalizePanelLoad() {
    var btn = document.getElementById("panel-expand-btn");
    if (btn) btn.disabled = false;
  }

  function closePanel() {
    var overlay = document.getElementById("detail-panel-overlay");
    var panel = document.getElementById("detail-panel");
    if (overlay) overlay.classList.remove("open");
    if (panel) panel.classList.remove("open");

    document.querySelectorAll(".install-row").forEach(function (r) {
      r.classList.remove("active");
    });

    var currentPath = window.location.pathname;
    if (currentPath.match(/\/installs\/[^\/]+/)) {
      var params = new URLSearchParams(window.location.search);
      var newUrl =
        basePath() +
        "/installs" +
        (params.toString() ? "?" + params.toString() : "");
      history.pushState({ panelClosed: true }, "", newUrl);
    }

    currentInstallId = null;
    historyLoaded = false;
    auditLoaded = false;

    if (panel) {
      panel.classList.remove("panel-full");
    }
    var expandIcon = document.getElementById("panel-expand-icon");
    var collapseIcon = document.getElementById("panel-collapse-icon");
    var expandBtn = document.getElementById("panel-expand-btn");
    if (expandIcon) expandIcon.classList.remove("hidden");
    if (collapseIcon) collapseIcon.classList.add("hidden");
    if (expandBtn) expandBtn.setAttribute("title", "Expand to full screen");

    document.body.style.overflow = "";
  }

  function toggleSize() {
    var panel = document.getElementById("detail-panel");
    var expandIcon = document.getElementById("panel-expand-icon");
    var collapseIcon = document.getElementById("panel-collapse-icon");
    var expandBtn = document.getElementById("panel-expand-btn");

    if (panel.classList.contains("panel-full")) {
      panel.classList.remove("panel-full");
      expandIcon.classList.remove("hidden");
      collapseIcon.classList.add("hidden");
      expandBtn.setAttribute("title", "Expand to full screen");
    } else {
      panel.classList.add("panel-full");
      expandIcon.classList.add("hidden");
      collapseIcon.classList.remove("hidden");
      expandBtn.setAttribute("title", "Resize to default size");
    }
  }

  function switchTab(tabName) {
    var tabs = ["overview", "history", "audit"];
    tabs.forEach(function (t) {
      var btn = document.getElementById("tab-" + t + "-btn");
      var pane = document.getElementById("tab-" + t);
      if (btn) btn.classList.toggle("active", t === tabName);
      if (pane) pane.classList.toggle("active", t === tabName);
    });

    if (
      tabName === "history" &&
      !historyLoaded &&
      currentInstallId &&
      typeof htmx !== "undefined"
    ) {
      htmx.ajax(
        "GET",
        basePath() + "/installs/" + currentInstallId + "/panel/history",
        { target: "#panel-content-history", swap: "innerHTML" }
      );
      historyLoaded = true;
    }

    if (
      tabName === "audit" &&
      !auditLoaded &&
      currentInstallId &&
      typeof htmx !== "undefined"
    ) {
      htmx.ajax(
        "GET",
        basePath() + "/installs/" + currentInstallId + "/panel/audit",
        { target: "#panel-content-audit", swap: "innerHTML" }
      );
      auditLoaded = true;
    }
  }

  // --- Event delegation ---

  document.addEventListener("click", function (e) {
    var target = e.target.closest("[data-panel-action]");
    if (!target) return;

    var action = target.dataset.panelAction;

    if (action === "close") {
      closePanel();
    } else if (action === "toggle-size") {
      toggleSize();
    } else if (action === "switch-tab") {
      switchTab(target.dataset.tab);
    }
  });

  // Close on overlay click
  document.addEventListener("click", function (e) {
    if (
      e.target.id === "detail-panel-overlay" &&
      e.target.classList.contains("open")
    ) {
      closePanel();
    }
  });

  // Close on ESC
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape") closePanel();
  });

  // Handle back/forward navigation
  window.addEventListener("popstate", function () {
    var pathMatch = window.location.pathname.match(/\/installs\/([^\/\?]+)/);

    if (pathMatch && pathMatch[1]) {
      var installId = pathMatch[1];
      var row = document.querySelector(
        '[data-install-id="' + installId + '"]'
      );
      if (row && currentInstallId !== installId && typeof htmx !== "undefined") {
        htmx.trigger(row, "click");
      }
    } else {
      if (currentInstallId) closePanel();
    }
  });

  // Auto-open panel on page load if URL has install ID
  document.addEventListener("DOMContentLoaded", function () {
    var pathMatch = window.location.pathname.match(/\/installs\/([^\/\?]+)/);
    if (pathMatch && pathMatch[1]) {
      var row = document.querySelector(
        '[data-install-id="' + pathMatch[1] + '"]'
      );
      if (row && typeof htmx !== "undefined") {
        htmx.trigger(row, "click");
      }
    }
  });

  // Expose for HTMX callbacks (hx-on attributes)
  window.preparePanel = preparePanel;
  window.finalizePanelLoad = finalizePanelLoad;
  window.closeInstallPanel = closePanel;
  window.switchPanelTab = switchTab;
  window.togglePanelSize = toggleSize;
})();
