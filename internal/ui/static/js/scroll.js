// Mark-read-on-scroll (opt-in): an unread list item that has scrolled fully
// above the top of the viewport is marked read. IDs are batched into one
// POST /entry/status request per second of scrolling.
(() => {
    "use strict";

    if (document.querySelector("main > article > section")) return; // entry page, not a list
    // Application root is derived from this script's URL (<base>/js/<checksum>/scroll.js).
    const statusURL = new URL("../../entry/status", document.currentScript.src);

    const pending = new Set();
    let timer = 0;

    async function flush() {
        const articles = [...pending];
        pending.clear();
        if (articles.length === 0) return;
        const body = new URLSearchParams();
        for (const article of articles) body.append("id", article.dataset.id);
        body.append("status", "read");
        const res = await fetch(statusURL, { method: "POST", body, redirect: "manual", keepalive: true });
        if (res.type !== "opaqueredirect" && !res.ok) throw new Error(`${statusURL}: HTTP ${res.status}`);
        for (const article of articles) article.dataset.status = "read";
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
