// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"net/http"
	"slices"
	"time"

	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
	"sss/internal/validator"
)

func (h *handler) showCategoryListPage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	categories, err := h.store.CategoriesWithFeedCount(user.ID, user.CategoriesSortingOrder)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	// sss: an empty Uncategorized is always hidden; other empty categories only on request.
	categories = slices.DeleteFunc(categories, func(c model.Category) bool {
		return *c.FeedCount == 0 && (user.HideEmptyCategories || c.Title == model.UncategorizedCategory)
	})
	h.render(w, r, "categories", map[string]any{"title": "Categories", "menu": "categories", "categories": categories})
}

// category returns the category from the route, writing a 404 or 500 response when it returns nil.
func (h *handler) category(w http.ResponseWriter, r *http.Request) *model.Category {
	category, err := h.store.Category(request.UserID(r), request.RouteInt64Param(r, "categoryID"))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return nil
	}
	if category == nil {
		response.HTMLNotFound(w, r)
	}
	return category
}

func (h *handler) showCreateCategoryPage(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, "category_form", map[string]any{"title": "New category", "menu": "categories", "category": &model.Category{}})
}

func (h *handler) saveCategory(w http.ResponseWriter, r *http.Request) {
	userID := request.UserID(r)
	req := &model.CategoryCreationRequest{Title: r.FormValue("title"), HideGlobally: r.FormValue("hide_globally") == "1"}
	if err := validator.ValidateCategoryCreation(h.store, userID, req); err != nil {
		h.render(w, r, "category_form", map[string]any{"title": "New category", "menu": "categories",
			"category": &model.Category{Title: req.Title, HideGlobally: req.HideGlobally}, "errorMessage": err.Error()})
		return
	}
	if _, err := h.store.CreateCategory(userID, req); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/categories"))
}

func (h *handler) showEditCategoryPage(w http.ResponseWriter, r *http.Request) {
	if category := h.category(w, r); category != nil {
		h.render(w, r, "category_form", map[string]any{"title": "Edit category: " + category.Title, "menu": "categories", "category": category})
	}
}

func (h *handler) updateCategory(w http.ResponseWriter, r *http.Request) {
	category := h.category(w, r)
	if category == nil {
		return
	}
	title, hide := r.FormValue("title"), r.FormValue("hide_globally") == "1"
	req := &model.CategoryModificationRequest{Title: &title, HideGlobally: &hide}
	if err := validator.ValidateCategoryModification(h.store, category.UserID, category.ID, req); err != nil {
		req.Patch(category)
		h.render(w, r, "category_form", map[string]any{"title": "Edit category", "menu": "categories", "category": category, "errorMessage": err.Error()})
		return
	}
	req.Patch(category)
	if err := h.store.UpdateCategory(category); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/category/%d/feeds", category.ID))
}

func (h *handler) removeCategory(w http.ResponseWriter, r *http.Request) {
	category := h.category(w, r)
	if category == nil {
		return
	}
	if err := h.store.RemoveCategory(category.UserID, category.ID); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/categories"))
}

func (h *handler) markCategoryAsRead(w http.ResponseWriter, r *http.Request) {
	category := h.category(w, r)
	if category == nil {
		return
	}
	if err := h.store.MarkCategoryAsRead(category.UserID, category.ID, time.Now()); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/categories"))
}

func (h *handler) showCategoryFeedsPage(w http.ResponseWriter, r *http.Request) {
	category := h.category(w, r)
	if category == nil {
		return
	}
	feeds, err := h.store.FeedsByCategoryWithCounters(category.UserID, category.ID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	h.render(w, r, "feeds", map[string]any{"title": category.Title, "menu": "categories", "category": category, "feeds": feeds})
}

// categoryFeed checks that the route feed belongs to the route category.
func (h *handler) categoryFeed(w http.ResponseWriter, r *http.Request) (categoryID, feedID int64, ok bool) {
	categoryID, feedID = request.RouteInt64Param(r, "categoryID"), request.RouteInt64Param(r, "feedID")
	exists, err := h.store.CategoryFeedExists(request.UserID(r), categoryID, feedID)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return 0, 0, false
	}
	if !exists {
		response.HTMLNotFound(w, r)
		return 0, 0, false
	}
	return categoryID, feedID, true
}

func (h *handler) removeCategoryFeed(w http.ResponseWriter, r *http.Request) {
	categoryID, feedID, ok := h.categoryFeed(w, r)
	if !ok {
		return
	}
	if err := h.store.RemoveFeed(request.UserID(r), feedID); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/category/%d/feeds", categoryID))
}

func (h *handler) markCategoryFeedAsRead(w http.ResponseWriter, r *http.Request) {
	categoryID, feedID, ok := h.categoryFeed(w, r)
	if !ok {
		return
	}
	userID := request.UserID(r)
	checkedAt, err := h.store.CheckedAt(userID, feedID)
	if err != nil {
		response.HTMLNotFound(w, r)
		return
	}
	if err := h.store.MarkFeedAsRead(userID, feedID, checkedAt); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/category/%d/feeds", categoryID))
}

func (h *handler) refreshCategoryEntriesPage(w http.ResponseWriter, r *http.Request) {
	h.refreshCategory(w, r, "/category/%d/entries")
}

func (h *handler) refreshCategoryFeedsPage(w http.ResponseWriter, r *http.Request) {
	h.refreshCategory(w, r, "/category/%d/feeds")
}

func (h *handler) refreshCategory(w http.ResponseWriter, r *http.Request, redirect string) {
	categoryID := request.RouteInt64Param(r, "categoryID")
	if h.refreshFeeds(w, r, categoryID) {
		response.HTMLRedirect(w, r, route(redirect, categoryID))
	}
}
