// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0
//
// Service worker: page navigations go to the network first; when the network
// fails, the cached offline page is shown instead. Nothing else is cached.
const CACHE = "offline-v1";
const OFFLINE_URL = new URL("offline", self.location).href;

self.addEventListener("install", (event) => {
    event.waitUntil(caches.open(CACHE).then((cache) => cache.add(new Request(OFFLINE_URL, { cache: "reload" }))));
    self.skipWaiting();
});

self.addEventListener("fetch", (event) => {
    if (event.request.mode !== "navigate") return;
    event.respondWith(fetch(event.request).catch(() => caches.match(OFFLINE_URL)));
});
