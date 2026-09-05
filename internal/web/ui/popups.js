/**
 * @param {{ rootSelector: string, close: () => void }[]} popups
 */
export function registerPopupDismiss(popups) {
  document.addEventListener("click", (e) => {
    for (const popup of popups) {
      if (!e.target.closest(popup.rootSelector)) popup.close();
    }
  });

  document.addEventListener("contextmenu", (e) => {
    const ctx = popups.find((p) => p.rootSelector === ".notes-ctx");
    if (!ctx) return;
    if (e.target.closest(".notes-ctx")) return;
    if (!e.target.closest(".notes-card")) ctx.close();
  });

  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape") return;
    for (const popup of popups) popup.close();
  });
}

/** @param {() => void} dismissOthers */
export function createDismissHook(dismissOthers) {
  return () => dismissOthers();
}
