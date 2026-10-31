// Copyright The Miniflux Authors. Modified by The sss Authors.
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"errors"
	"net/http"
	"strconv"

	"sss/internal/http/request"
	"sss/internal/http/response"
	"sss/internal/model"
	"sss/internal/validator"
)

// adminOnly rejects non-admin users with 403.
func adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if user := currentUser(r); user == nil || !user.IsAdmin {
			response.HTMLForbidden(w, r)
			return
		}
		next(w, r)
	}
}

func (h *handler) showUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.Users()
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	users.UseTimezone(currentUser(r).Timezone)
	h.renderAccount(w, r, "account_users", map[string]any{"title": "Users", "users": users})
}

func (h *handler) showCreateUser(w http.ResponseWriter, r *http.Request) {
	h.renderAccount(w, r, "account_user_create", map[string]any{"title": "New user"})
}

func (h *handler) saveUser(w http.ResponseWriter, r *http.Request) {
	req := &model.UserCreationRequest{
		Username: r.FormValue("username"),
		Password: r.FormValue("password"),
		IsAdmin:  r.FormValue("is_admin") != "",
	}
	err := validator.ValidateUserCreationWithPassword(h.store, req)
	if err == nil && req.Password != r.FormValue("confirmation") {
		err = errors.New("Passwords are not the same.")
	}
	if err != nil {
		h.renderAccount(w, r, "account_user_create", map[string]any{"title": "New user", "form": req, "errorMessage": err.Error()})
		return
	}
	if _, err := h.store.CreateUser(req); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/users"))
}

func (h *handler) selectedUser(w http.ResponseWriter, r *http.Request) *model.User {
	id, _ := strconv.ParseInt(r.PathValue("userID"), 10, 64)
	user, err := h.store.UserByID(id)
	if err != nil {
		response.HTMLServerError(w, r, err)
		return nil
	}
	if user == nil {
		response.HTMLNotFound(w, r)
	}
	return user
}

func (h *handler) showEditUser(w http.ResponseWriter, r *http.Request) {
	if user := h.selectedUser(w, r); user != nil {
		h.renderAccount(w, r, "account_user_edit", map[string]any{"title": "Edit user", "selected": user, "form": user})
	}
}

func (h *handler) updateUser(w http.ResponseWriter, r *http.Request) {
	selected := h.selectedUser(w, r)
	if selected == nil {
		return
	}
	username, password := r.FormValue("username"), r.FormValue("password")
	isAdmin := r.FormValue("is_admin") != ""
	changes := &model.UserModificationRequest{Username: &username, IsAdmin: &isAdmin}
	if password != "" {
		changes.Password = &password
	}
	err := validator.ValidateUserModification(h.store, selected.ID, changes)
	if err == nil && password != r.FormValue("confirmation") {
		err = errors.New("Passwords are not the same.")
	}
	updated := *selected
	changes.Patch(&updated)
	if err != nil {
		h.renderAccount(w, r, "account_user_edit", map[string]any{"title": "Edit user", "selected": selected, "form": &updated, "errorMessage": err.Error()})
		return
	}
	if err := h.store.UpdateUser(&updated); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	if password != "" {
		// Administrators editing their own account keep their current session.
		if err := h.store.RemoveUserWebSessionsExcept(selected.ID, request.WebSession(r).ID); err != nil {
			response.HTMLServerError(w, r, err)
			return
		}
	}
	response.HTMLRedirect(w, r, route("/users"))
}

func (h *handler) removeUser(w http.ResponseWriter, r *http.Request) {
	selected := h.selectedUser(w, r)
	if selected == nil {
		return
	}
	if selected.ID == request.UserID(r) {
		response.HTMLBadRequest(w, r, errors.New("you cannot remove yourself"))
		return
	}
	if err := h.store.RemoveUser(selected.ID); err != nil {
		response.HTMLServerError(w, r, err)
		return
	}
	response.HTMLRedirect(w, r, route("/users"))
}
