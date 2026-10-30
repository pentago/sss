// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0
//
// Keyboard shortcuts (opt-in). Press "?" for the list. Acts only through the
// links and forms already in the page, so every shortcut has a no-JS equivalent.
(() => {
    "use strict";

    // Application root, derived from this script's URL (<base>/js/<checksum>/shortcuts.js).
    const base = new URL("../../", document.currentScript.src);
    const $ = (selector, root = document) => root.querySelector(selector);
    const isEntryPage = () => $("main > article > section") !== null;
    const listItems = () => [...document.querySelectorAll("main > article[data-id]")];
    const selectedItem = () => document.activeElement?.closest("main > article[data-id]") ?? null;
    const currentEntry = () => (isEntryPage() ? $("main > article") : selectedItem());
    const follow = (link) => { if (link) location.href = link.href; };
    const goTo = (path) => follow($(`nav[aria-label="Main"] a[href$="${path}"]`));
    const goRel = (rel) => follow($(`main a[rel~="${rel}"]`));

    // Submit a form in the background (urlencoded, like the browser would); the server answers with a redirect.
    async function post(form) {
        const res = await fetch(form.action, { method: "POST", body: new URLSearchParams(new FormData(form)), redirect: "manual" });
        if (res.type !== "opaqueredirect" && !res.ok) throw new Error(`${form.action}: HTTP ${res.status}`);
    }

    async function toggleRead(article) {
        const form = article && $('form[action$="/entry/status"]', article);
        if (!form) return;
        const status = form.elements.namedItem("status");
        await post(form);
        article.dataset.status = status.value;
        status.value = status.value === "read" ? "unread" : "read";
        $("button", form).textContent = status.value === "read" ? "Mark as read" : "Mark as unread";
    }

    async function toggleStar(article) {
        const form = article && $('form[action*="/entry/star/"]', article);
        if (!form) return;
        await post(form);
        const starred = article.dataset.starred !== "true";
        article.dataset.starred = String(starred);
        $("button", form).setAttribute("aria-pressed", String(starred));
    }

    async function save(article) {
        const form = article && $('form[action*="/entry/save/"]', article);
        if (!form) return;
        await post(form);
        $("button", form).disabled = true;
    }

    // Select a list item relative to the current one (wraps around).
    function select(offset) {
        const items = listItems();
        if (items.length === 0) return;
        const index = items.indexOf(selectedItem());
        focusItem(index < 0 ? items[0] : items[(index + offset + items.length) % items.length]);
    }

    function focusItem(item) {
        item.tabIndex = -1;
        item.focus({ preventScroll: true });
        item.scrollIntoView({ block: "nearest" });
    }

    // First/last item; already there: previous/next page. On entry pages, scroll to top/bottom.
    function edge(last) {
        if (isEntryPage()) {
            window.scrollTo(0, last ? document.body.scrollHeight : 0);
            return;
        }
        const items = listItems();
        const target = last ? items.at(-1) : items[0];
        if (!target || selectedItem() === target) goRel(last ? "next" : "prev");
        else focusItem(target);
    }

    // Next/previous item on list pages, next/previous entry on entry pages.
    const move = (offset) => (isEntryPage() ? goRel(offset > 0 ? "next" : "prev") : select(offset));

    const open = (link, newTab) => {
        if (!link) return;
        if (newTab) window.open(link.href, "_blank", "noopener,noreferrer");
        else location.href = link.href;
    };

    async function openOriginal(newTab) {
        const article = currentEntry();
        if (!article) return;
        if (isEntryPage()) {
            open($("header h1 a", article), newTab);
            return;
        }
        const link = $('footer a[rel~="noreferrer"]', article);
        if (!link) return;
        if (!newTab) {
            if (article.dataset.status === "unread") await toggleRead(article);
            open(link, false);
            return;
        }
        open(link, true);
        // On lists other than Starred, mark as read and move on.
        if ($('nav[aria-label="Main"] [aria-current="page"]')?.getAttribute("href").endsWith("/starred")) return;
        select(1);
        if (article.dataset.status === "unread") toggleRead(article);
    }

    const commentsLink = (article) => article && [...article.querySelectorAll("a")].find((a) => a.textContent === "Comments");

    async function refreshAll() {
        await fetch(new URL("feeds/refresh", base), { method: "POST", redirect: "manual" });
        location.reload();
    }

    // "#" on a feed page: its single feed removal form.
    function removeFeed() {
        const forms = document.querySelectorAll('main form[action*="/feed/"][action$="/remove"]');
        if (forms.length === 1 && confirm("Remove this feed?")) forms[0].requestSubmit();
    }

    function readAndMove(offset) {
        const article = currentEntry();
        if (!article) return;
        toggleRead(article);
        if (!isEntryPage()) select(offset);
    }

    const shortcuts = [
        [["g u"], "Go to unread", () => goTo("/unread")],
        [["g b"], "Go to starred", () => goTo("/starred")],
        [["g h"], "Go to history", () => goTo("/history")],
        [["g f"], "Go to feeds", () => goTo("/feeds")],
        [["g c"], "Go to categories", () => goTo("/categories")],
        [["g s"], "Go to settings", () => goTo("/settings")],
        [["g g"], "First item (previous page)", () => edge(false)],
        [["G"], "Last item (next page)", () => edge(true)],
        [["j", "n", "ArrowRight"], "Next item", () => move(1)],
        [["k", "p", "ArrowLeft"], "Previous item", () => move(-1)],
        [["l"], "Next page", () => goRel("next")],
        [["h"], "Previous page", () => goRel("prev")],
        [["z t"], "Scroll selected item to top", () => currentEntry()?.scrollIntoView({ block: "start" })],
        [["o", "Enter"], "Open selected item", () => follow(selectedItem() && $("h2 a", selectedItem()))],
        [["v"], "Open original link in new tab", () => openOriginal(true)],
        [["V"], "Open original link in current tab", () => openOriginal(false)],
        [["c"], "Open comments link in new tab", () => open(commentsLink(currentEntry()), true)],
        [["C"], "Open comments link in current tab", () => open(commentsLink(currentEntry()), false)],
        [["m"], "Toggle read, select next", () => readAndMove(1)],
        [["M"], "Toggle read, select previous", () => readAndMove(-1)],
        [["f"], "Toggle star", () => toggleStar(currentEntry())],
        [["s"], "Save entry", () => save(currentEntry())],
        [["d"], "Fetch original content", () => $('main > article form[action*="/entry/download/"]')?.requestSubmit()],
        [["F"], "Go to feed", () => follow(currentEntry() && $('header p a[href*="/feed/"]', currentEntry()))],
        [["R"], "Refresh all feeds", refreshAll],
        [["+"], "Add subscription", () => { location.href = new URL("subscribe", base); }],
        [["#"], "Remove feed", removeFeed],
        [["a"], "Toggle attachments", () => { const d = $("main > article > footer > details"); if (d) d.open = !d.open; }],
        [["A"], "Read page", () => $('nav[aria-label="Page actions"] form[action$="/entry/status"]')?.requestSubmit()],
        [["/"], "Go to search", () => goTo("/search")],
        [["?"], "Show this help", showHelp],
    ];

    const actions = new Map();
    for (const [keys, , action] of shortcuts) {
        for (const key of keys) actions.set(key, action);
    }

    let help;
    function showHelp() {
        if (!help) {
            const el = (tag, text) => Object.assign(document.createElement(tag), { textContent: text ?? "" });
            help = el("dialog");
            help.setAttribute("aria-label", "Keyboard shortcuts");
            const list = el("dl");
            for (const [keys, description] of shortcuts) {
                const term = el("dt");
                keys.forEach((key, i) => term.append(i ? " / " : "", el("kbd", key)));
                list.append(term, el("dd", description));
            }
            const close = el("form");
            close.method = "dialog";
            close.append(el("button", "Close"));
            help.append(el("h2", "Keyboard shortcuts"), list, close);
            document.body.append(help);
        }
        help.showModal();
    }

    let prefix = "";
    document.addEventListener("keydown", (event) => {
        if (event.ctrlKey || event.altKey || event.metaKey || event.key === "Shift") return;
        const target = event.target;
        if (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)) return;
        if (event.key === "Enter" && /^(A|BUTTON|SUMMARY)$/.test(target.tagName)) return;
        if ($("dialog[open]")) return;

        const key = prefix ? `${prefix} ${event.key}` : event.key;
        prefix = "";
        const action = actions.get(key);
        if (action) {
            event.preventDefault();
            action();
        } else if (key === "g" || key === "z") {
            event.preventDefault();
            prefix = key;
        }
    });
})();
