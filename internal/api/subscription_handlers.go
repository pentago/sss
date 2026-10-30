// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package api

import (
	json_parser "encoding/json"
	"net/http"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
	"sss/internal/reader/fetcher"
	"sss/internal/reader/subscription"
	"sss/internal/validator"
)

func (h *handler) discoverSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	var subscriptionDiscoveryRequest model.SubscriptionDiscoveryRequest
	if err := json_parser.NewDecoder(r.Body).Decode(&subscriptionDiscoveryRequest); err != nil {
		response.JSONBadRequest(w, r, err)
		return
	}

	if validationErr := validator.ValidateSubscriptionDiscovery(&subscriptionDiscoveryRequest); validationErr != nil {
		response.JSONBadRequest(w, r, validationErr)
		return
	}

	var rssbridgeURL string
	var rssbridgeToken string
	intg, err := h.store.Integration(request.UserID(r))
	if err == nil && intg != nil && intg.RSSBridgeEnabled {
		rssbridgeURL = intg.RSSBridgeURL
		rssbridgeToken = intg.RSSBridgeToken
	}

	requestBuilder := fetcher.NewRequestBuilder().
		WithTimeout(config.Opts.HTTPClientTimeout).
		WithCustomFeedProxyURL(subscriptionDiscoveryRequest.ProxyURL).
		WithCustomApplicationProxyURL(config.Opts.HTTPClientProxy).
		UseCustomApplicationProxyURL(subscriptionDiscoveryRequest.FetchViaProxy).
		WithUserAgent(subscriptionDiscoveryRequest.UserAgent, config.Opts.HTTPClientUserAgent).
		WithCookie(subscriptionDiscoveryRequest.Cookie, subscriptionDiscoveryRequest.URL).
		WithUsernameAndPassword(subscriptionDiscoveryRequest.Username, subscriptionDiscoveryRequest.Password).
		IgnoreTLSErrors(subscriptionDiscoveryRequest.AllowSelfSignedCertificates).
		DisableHTTP2(subscriptionDiscoveryRequest.DisableHTTP2)

	subscriptions, localizedError := subscription.NewSubscriptionFinder(requestBuilder).FindSubscriptions(
		subscriptionDiscoveryRequest.URL,
		rssbridgeURL,
		rssbridgeToken,
	)

	if localizedError != nil {
		response.JSONServerError(w, r, localizedError)
		return
	}

	if len(subscriptions) == 0 {
		response.JSONNotFound(w, r)
		return
	}

	response.JSON(w, r, subscriptions)
}
