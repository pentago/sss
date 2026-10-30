// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0
//
// Touch gestures (opt-in):
// - <body data-entry-swipe>: swipe a list item sideways to toggle read/unread.
// - <body data-gesture-nav="tap|swipe">: on an entry page, double-tap the
//   left/right half or swipe right/left to go to the previous/next entry.
(() => {
    "use strict";

    const $ = (selector, root = document) => root.querySelector(selector);
    const entry = $("main > article > section")?.parentElement;
    const follow = (rel) => { const link = $(`main a[rel~="${rel}"]`); if (link) location.href = link.href; };
    const SWIPE = 75; // px of horizontal travel that counts as a swipe

    async function toggleRead(article) {
        const form = $('form[action$="/entry/status"]', article);
        if (!form) return;
        const status = form.elements.namedItem("status");
        const res = await fetch(form.action, { method: "POST", body: new URLSearchParams(new FormData(form)), redirect: "manual" });
        if (res.type !== "opaqueredirect" && !res.ok) throw new Error(`${form.action}: HTTP ${res.status}`);
        article.dataset.status = status.value;
        status.value = status.value === "read" ? "unread" : "read";
        $("button", form).textContent = status.value === "read" ? "Mark as read" : "Mark as unread";
    }

    // Horizontal travel of a gesture, or 0 while it still looks like a vertical scroll.
    const travel = (start, touch, locked) => {
        const dx = touch.clientX - start.clientX;
        return locked || (Math.abs(dx) > 30 && Math.abs(touch.clientY - start.clientY) < 70) ? dx : 0;
    };

    if ("entrySwipe" in document.body.dataset && !entry) {
        for (const article of document.querySelectorAll("main > article[data-status]")) {
            let start = null;
            let locked = false;
            article.addEventListener("touchstart", (event) => {
                if (event.touches.length !== 1) return;
                start = event.touches[0];
                locked = false;
                article.style.transition = "none";
            }, { passive: true });
            article.addEventListener("touchmove", (event) => {
                if (!start || event.touches.length !== 1) return;
                const dx = travel(start, event.touches[0], locked);
                if (dx === 0) return;
                locked = true;
                event.preventDefault();
                const abs = Math.abs(dx);
                const damped = abs > SWIPE ? SWIPE + Math.sqrt(abs - SWIPE) : abs;
                article.style.transform = `translateX(${Math.sign(dx) * damped}px)`;
            });
            const end = (event) => {
                if (start && locked) {
                    if (event.type === "touchend" && Math.abs(travel(start, event.changedTouches[0], true)) > SWIPE) {
                        toggleRead(article);
                    }
                    article.style.transition = "transform 0.15s";
                    article.style.transform = "";
                }
                start = null;
            };
            article.addEventListener("touchend", end, { passive: true });
            article.addEventListener("touchcancel", end, { passive: true });
        }
    }

    const mode = document.body.dataset.gestureNav;
    if (entry && mode === "swipe") {
        let start = null;
        let time = 0;
        entry.addEventListener("touchstart", (event) => {
            start = event.touches.length === 1 ? event.touches[0] : null;
            time = Date.now();
        }, { passive: true });
        entry.addEventListener("touchend", (event) => {
            if (!start || Date.now() - time > 1000) return;
            const dx = travel(start, event.changedTouches[0], false);
            if (Math.abs(dx) > SWIPE) follow(dx > 0 ? "prev" : "next");
            start = null;
        }, { passive: true });
    } else if (entry && mode === "tap") {
        let lastX = -1;
        let lastTime = 0;
        entry.addEventListener("touchmove", () => { lastX = -1; }, { passive: true });
        entry.addEventListener("touchend", (event) => {
            const x = event.changedTouches[0].clientX;
            const now = Date.now();
            const half = window.innerWidth / 2;
            if (lastX >= 0 && now - lastTime <= 200 && (lastX >= half) === (x >= half)) {
                follow(x >= half ? "next" : "prev");
                lastX = -1;
                return;
            }
            lastX = x;
            lastTime = now;
        }, { passive: true });
    }
})();
