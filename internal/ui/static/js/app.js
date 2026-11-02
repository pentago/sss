// Always loaded. A page restored from the back/forward cache shows stale
// state (e.g. an entry read since is still listed), so reload it.
addEventListener("pageshow", (event) => {
    if (event.persisted) location.reload();
});

// "Read page" and "Read all" post in the background, then load the next page
// in place of the current history entry: a normal submit would add an entry,
// and "back" would return to the list as it was before it was marked read.
// "Read page" loads the same list again, "Read all" on a feed or category goes
// up to its data-redirect list (the server's redirect target). Not history.back():
// after "Go to top" the previous entry is this same list. Unlike reload(), this
// starts at the top (the bottom nav's "Read page" would otherwise land at the
// bottom of the next page); the hash is dropped so a "#top" URL is fetched
// again instead of only scrolled.
addEventListener("submit", (event) => {
    const form = event.target;
    if (!form.matches('form[action$="/mark-all-as-read"], nav:is([aria-label="Page actions"], [aria-label="Bottom page actions"]) form[action$="/entry/status"]')) return;
    event.preventDefault();
    const next = new URL(form.dataset.redirect || location.href, location.href);
    next.hash = "";
    fetch(form.action, { method: "POST", body: new URLSearchParams(new FormData(form)), redirect: "manual" })
        .then((res) => (res.type === "opaqueredirect" ? location.replace(next) : form.submit()), () => form.submit());
});
