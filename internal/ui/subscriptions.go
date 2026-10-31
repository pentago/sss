// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"net/http"
	"regexp"
	"strconv"

	"sss/internal/config"
	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
	"sss/internal/reader/fetcher"
	feedHandler "sss/internal/reader/handler"
	"sss/internal/reader/subscription"
	"sss/internal/validator"
)

// Best effort URL extraction from text shared through the Web Share Target API.
var urlRe = regexp.MustCompile(`(?i)(?:https?://)?[0-9a-z.]+[.][a-z]+(?::[0-9]+)?(?:/[^ ]+|/)?`)

func (h *handler) showAddSubscriptionPage(w http.ResponseWriter, r *http.Request) {
	h.renderAddSubscription(w, r, &model.FeedCreationRequest{}, "")
}

func (h *handler) bookmarklet(w http.ResponseWriter, r *http.Request) {
	feedURL := request.QueryStringParam(r, "uri", "")
	if text := request.QueryStringParam(r, "text", ""); text != "" && feedURL == "" {
		feedURL = urlRe.FindString(text)
	}
	h.renderAddSubscription(w, r, &model.FeedCreationRequest{FeedURL: feedURL}, "")
}

func (h *handler) renderAddSubscription(w http.ResponseWriter, r *http.Request, form *model.FeedCreationRequest, errorMessage string) {
	categories, err := h.store.Categories(request.UserID(r))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	h.render(w, r, "subscribe", map[string]any{"title": "New subscription", "menu": "feeds", "form": form, "categories": categories,
		"errorMessage": errorMessage, "defaultUserAgent": currentUser(r).DefaultUserAgent(), "hasProxyConfigured": config.Opts.HTTPClientProxy != nil})
}

// subscriptionForm reads the subscription form fields.
func subscriptionForm(r *http.Request) *model.FeedCreationRequest {
	categoryID, _ := strconv.ParseInt(r.FormValue("category_id"), 10, 64)
	return &model.FeedCreationRequest{
		FeedURL:                     r.FormValue("url"),
		CategoryID:                  categoryID,
		Crawler:                     r.FormValue("crawler") == "1",
		IgnoreEntryUpdates:          r.FormValue("ignore_entry_updates") == "1",
		AllowSelfSignedCertificates: r.FormValue("allow_self_signed_certificates") == "1",
		FetchViaProxy:               r.FormValue("fetch_via_proxy") == "1",
		DisableHTTP2:                r.FormValue("disable_http2") == "1",
		UserAgent:                   r.FormValue("user_agent"),
		Cookie:                      r.FormValue("cookie"),
		Username:                    r.FormValue("feed_username"),
		Password:                    r.FormValue("feed_password"),
		ScraperRules:                r.FormValue("scraper_rules"),
		RewriteRules:                r.FormValue("rewrite_rules"),
		UrlRewriteRules:             r.FormValue("urlrewrite_rules"),
		BlocklistRules:              r.FormValue("blocklist_rules"),
		KeeplistRules:               r.FormValue("keeplist_rules"),
		BlockFilterEntryRules:       r.FormValue("block_filter_entry_rules"),
		KeepFilterEntryRules:        r.FormValue("keep_filter_entry_rules"),
		ProxyURL:                    r.FormValue("proxy_url"),
	}
}

// submitSubscription discovers the feeds of the submitted URL: a single feed
// is created directly, several are offered on the choose page.
func (h *handler) submitSubscription(w http.ResponseWriter, r *http.Request) {
	form := subscriptionForm(r)
	if err := validator.ValidateFeedCreation(h.store, request.UserID(r), form); err != nil {
		h.renderAddSubscription(w, r, form, err.Error())
		return
	}

	var rssBridgeURL, rssBridgeToken string
	if intg, err := h.store.Integration(request.UserID(r)); err == nil && intg != nil && intg.RSSBridgeEnabled {
		rssBridgeURL, rssBridgeToken = intg.RSSBridgeURL, intg.RSSBridgeToken
	}
	finder := subscription.NewSubscriptionFinder(fetcher.NewRequestBuilder().
		WithTimeout(config.Opts.HTTPClientTimeout).
		WithCustomFeedProxyURL(form.ProxyURL).
		WithCustomApplicationProxyURL(config.Opts.HTTPClientProxy).
		UseCustomApplicationProxyURL(form.FetchViaProxy).
		WithUserAgent(form.UserAgent, currentUser(r).DefaultUserAgent()).
		WithCookie(form.Cookie, form.FeedURL).
		WithUsernameAndPassword(form.Username, form.Password).
		IgnoreTLSErrors(form.AllowSelfSignedCertificates).
		DisableHTTP2(form.DisableHTTP2))
	subscriptions, err := finder.FindSubscriptions(form.FeedURL, rssBridgeURL, rssBridgeToken)
	if err != nil {
		h.renderAddSubscription(w, r, form, err.Error())
		return
	}

	switch {
	case len(subscriptions) == 0:
		h.renderAddSubscription(w, r, form, "Unable to find any feed.")
	case len(subscriptions) == 1:
		form.FeedURL = subscriptions[0].URL
		var feed *model.Feed
		if finder.IsFeedAlreadyDownloaded() {
			info := finder.FeedResponseInfo()
			info.FeedCreationRequest = *form
			feed, err = feedHandler.CreateFeedFromSubscriptionDiscovery(h.store, request.UserID(r), info)
		} else {
			feed, err = feedHandler.CreateFeed(h.store, request.UserID(r), form)
		}
		if err != nil {
			h.renderAddSubscription(w, r, form, err.Error())
			return
		}
		response.HTMLRedirect(w, r, route("/feed/%d/entries", feed.ID))
	default:
		h.render(w, r, "subscribe_choose", map[string]any{"title": "Choose a subscription", "menu": "feeds", "form": form, "subscriptions": subscriptions})
	}
}

// chooseSubscription creates the feed picked on the choose page.
func (h *handler) chooseSubscription(w http.ResponseWriter, r *http.Request) {
	form := subscriptionForm(r)
	if err := validator.ValidateFeedCreation(h.store, request.UserID(r), form); err != nil {
		h.renderAddSubscription(w, r, form, err.Error())
		return
	}
	feed, err := feedHandler.CreateFeed(h.store, request.UserID(r), form)
	if err != nil {
		h.renderAddSubscription(w, r, form, err.Error())
		return
	}
	response.HTMLRedirect(w, r, route("/feed/%d/entries", feed.ID))
}
