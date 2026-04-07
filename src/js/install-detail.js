// install-detail.js — Install detail page utilities.
// Tab navigation is now handled by page routing (sidebar links).
// This file retains secondary panel and config helpers.
(function () {
  function getConfig() {
    var el = document.getElementById("install-detail-page-config");
    if (!el) return { basePath: "", installId: "" };
    return {
      basePath: el.dataset.basePath || "",
      installId: el.dataset.installId || "",
    };
  }

  // Expose for external use
  window.getInstallConfig = getConfig;
})();
