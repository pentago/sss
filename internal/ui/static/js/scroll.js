// Mark-read-on-scroll (opt-in): an unread list item that has scrolled fully
// above the top of the viewport is marked read. IDs are batched into one
// POST /entry/status request per second of scrolling.
(() => {
    "use strict";

    if (document.querySelector("main > article > section")) return; // entry page, not a list
    const statusForm = document.querySelector('form[action$="/entry/status"]');
    if (!statusForm) return;

    const pending = new Set();
    let timer = 0;

    async function flush() {
        const articles = [...pending];
        pending.clear();
        if (articles.length === 0) return;
        const body = new URLSearchParams();
        for (const article of articles) body.append("id", article.dataset.id);
        body.append("status", "read");
        const res = await fetch(statusForm.action, { method: "POST", body, redirect: "manual", keepalive: true });
        if (res.type !== "opaqueredirect" && !res.ok) throw new Error(`${statusForm.action}: HTTP ${res.status}`);
        for (const article of articles) {
            article.dataset.status = "read";
            // Keep the item's own toggle form consistent with the new state.
            const form = article.querySelector('form[action$="/entry/status"]');
            const status = form?.elements.namedItem("status");
            if (status) {
                status.value = "unread";
                form.querySelector("button").textContent = "Mark as unread";
            }
        }
    }

    const observer = new IntersectionObserver((records) => {
        for (const { target, isIntersecting, boundingClientRect } of records) {
            if (isIntersecting || boundingClientRect.bottom > 0 || target.dataset.status !== "unread") continue;
            observer.unobserve(target);
            pending.add(target);
            clearTimeout(timer);
            timer = setTimeout(flush, 1000);
        }
    });
    document.querySelectorAll('main > article[data-status="unread"]').forEach((article) => observer.observe(article));
    addEventListener("pagehide", flush);
})();
