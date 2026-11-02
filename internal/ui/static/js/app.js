// Always loaded. A page restored from the back/forward cache shows stale
// state (e.g. an entry read since is still listed), so reload it.
addEventListener("pageshow", (event) => {
    if (event.persisted) location.reload();
});
