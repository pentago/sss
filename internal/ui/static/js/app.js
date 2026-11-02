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

// Long-press the blank part of an entry list item (not a link): the item is
// highlighted and its title reads "▲ Mark all above as read" (data-mark-above,
// see base.css). Tapping it marks the unread entries listed above it as read,
// in place and without confirmation; scrolling or a tap elsewhere disarms it.
if (!document.querySelector("main > article > section")) {
    const statusURL = new URL("../../entry/status", document.currentScript.src);
    let timer = 0, start = null, armed = null, fresh = false;
    const disarm = () => { clearTimeout(timer); delete armed?.dataset.markAbove; armed = null; };
    addEventListener("touchstart", (event) => {
        const article = event.target.closest("main > article[data-status]");
        if (article && article === armed) return;
        disarm();
        if (!article || event.target.closest("a, button") || event.touches.length !== 1) return;
        start = event.touches[0];
        timer = setTimeout(() => {
            const items = [...document.querySelectorAll("main > article[data-status]")];
            if (!items.slice(0, items.indexOf(article)).some((a) => a.dataset.status === "unread")) return;
            armed = article;
            armed.dataset.markAbove = "";
            fresh = true;
        }, 500);
    }, { passive: true });
    addEventListener("touchmove", (event) => {
        const t = event.touches[0];
        if (start && Math.hypot(t.clientX - start.clientX, t.clientY - start.clientY) > 10) clearTimeout(timer);
    }, { passive: true });
    // The release of the arming press must not count as the confirming tap.
    addEventListener("touchend", (event) => {
        clearTimeout(timer);
        if (fresh) event.preventDefault();
        fresh = false;
    });
    addEventListener("touchcancel", () => { clearTimeout(timer); fresh = false; }, { passive: true });
    addEventListener("scroll", disarm, { passive: true });
    // Android opens its own menu or a text selection on a long press.
    addEventListener("contextmenu", (event) => {
        if (event.target.closest("main > article[data-status]") && !event.target.closest("a")) event.preventDefault();
    });
    // A tap anywhere on the armed item (its title link included) marks the entries above.
    addEventListener("click", async (event) => {
        if (!armed || !armed.contains(event.target)) return;
        event.preventDefault();
        const items = [...document.querySelectorAll("main > article[data-status]")];
        const above = items.slice(0, items.indexOf(armed)).filter((a) => a.dataset.status === "unread");
        disarm();
        const body = new URLSearchParams(above.map((a) => ["id", a.dataset.id]));
        body.append("status", "read");
        const res = await fetch(statusURL, { method: "POST", body, redirect: "manual" });
        if (res.type === "opaqueredirect" || res.ok) for (const a of above) a.dataset.status = "read";
    });
}
