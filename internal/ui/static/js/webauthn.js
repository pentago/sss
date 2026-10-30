// Copyright The Miniflux Authors. Modified by The sss Authors.
// Derived from Miniflux (https://github.com/miniflux/v2).
// SPDX-License-Identifier: Apache-2.0
//
// Passkeys (opt-in, WEBAUTHN=1). Enables <button data-webauthn="register">
// and <button data-webauthn="login">, which the page should render disabled.
// A username input with autocomplete="username webauthn" also gets passkey
// autofill (conditional mediation).
(() => {
    "use strict";

    if (typeof PublicKeyCredential === "undefined" || !document.currentScript) return;

    // Application root, derived from this script's URL (<base>/js/<checksum>/webauthn.js).
    const base = new URL("../../", document.currentScript.src);
    const url = (path) => new URL(path, base);

    const decode = (value) => Uint8Array.from(atob(value.replace(/-/g, "+").replace(/_/g, "/")), (c) => c.charCodeAt(0));
    const encode = (buffer) => btoa(String.fromCharCode(...new Uint8Array(buffer ?? [])))
        .replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");

    async function getJSON(path) {
        const res = await fetch(url(path));
        if (!res.ok) throw new Error(`${path}: HTTP ${res.status}`);
        return res.json();
    }

    async function postJSON(path, data) {
        const res = await fetch(url(path), {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(data),
        });
        if (!res.ok) throw new Error(`${path}: HTTP ${res.status}`);
        return res.json();
    }

    async function register() {
        const options = await getJSON("webauthn/register/begin");
        const key = options.publicKey;
        key.challenge = decode(key.challenge);
        key.user.id = decode(key.user.id);
        key.excludeCredentials?.forEach((credential) => { credential.id = decode(credential.id); });

        const credential = await navigator.credentials.create(options);
        const result = await postJSON("webauthn/register/finish", {
            id: credential.id,
            rawId: encode(credential.rawId),
            type: credential.type,
            response: {
                attestationObject: encode(credential.response.attestationObject),
                clientDataJSON: encode(credential.response.clientDataJSON),
            },
        });
        location.href = result.redirect;
    }

    async function login(signal) {
        const options = await getJSON("webauthn/login/begin");
        const key = options.publicKey;
        key.challenge = decode(key.challenge);
        key.allowCredentials?.forEach((credential) => { credential.id = decode(credential.id); });
        if (signal) {
            options.signal = signal;
            options.mediation = "conditional";
        }

        const assertion = await navigator.credentials.get(options);
        if (!assertion) return;
        const finish = new URL("webauthn/login/finish", base);
        const redirect = new URLSearchParams(location.search).get("redirect_url");
        if (redirect) finish.searchParams.set("redirect_url", redirect);
        const result = await postJSON(finish, {
            id: assertion.id,
            rawId: encode(assertion.rawId),
            type: assertion.type,
            response: {
                authenticatorData: encode(assertion.response.authenticatorData),
                clientDataJSON: encode(assertion.response.clientDataJSON),
                signature: encode(assertion.response.signature),
                userHandle: encode(assertion.response.userHandle),
            },
        });
        location.href = result.redirect;
    }

    // Errors are shown in a <p role="alert"> right before the button.
    let shownError = null;
    function showError(button, err) {
        if (err instanceof DOMException && err.name === "AbortError") return;
        shownError?.remove();
        shownError = Object.assign(document.createElement("p"), { textContent: `Passkey error: ${err.message ?? err}` });
        shownError.setAttribute("role", "alert");
        button.before(shownError);
    }

    const registerButton = document.querySelector('button[data-webauthn="register"]');
    if (registerButton) {
        registerButton.disabled = false;
        registerButton.addEventListener("click", (event) => {
            event.preventDefault();
            register().catch((err) => showError(registerButton, err));
        });
    }

    const loginButton = document.querySelector('button[data-webauthn="login"]');
    if (loginButton) {
        const autofill = new AbortController();
        loginButton.disabled = false;
        loginButton.addEventListener("click", (event) => {
            event.preventDefault();
            autofill.abort();
            login().catch((err) => showError(loginButton, err));
        });

        if (document.querySelector('input[autocomplete~="webauthn"]')) {
            PublicKeyCredential.isConditionalMediationAvailable?.().then((available) => {
                if (available) login(autofill.signal).catch((err) => showError(loginButton, err));
            });
        }
    }
})();
