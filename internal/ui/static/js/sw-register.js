// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0
//
// Service worker registration (opt-in). The worker lives at the application
// root; derive it from this script's URL (<base>/js/<checksum>/sw-register.js)
// so a BASE_URL path prefix keeps working. The CSP requires Trusted Types, so
// the script URL goes through the "url" policy where the API exists.
if ("serviceWorker" in navigator && document.currentScript) {
    const src = new URL("../../service-worker.js", document.currentScript.src).href;
    const policy = window.trustedTypes?.createPolicy("url", { createScriptURL: (url) => url });
    navigator.serviceWorker.register(policy ? policy.createScriptURL(src) : src);
}
