// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-2021 Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public Licensee as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public Licensee for more details.
//
// You should have received a copy of the GNU Affero General Public Licensee
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package models

import (
	"time"

	"code.vikunja.io/api/pkg/db"

	"code.vikunja.io/api/pkg/events"
	user2 "code.vikunja.io/api/pkg/user"
	"code.vikunja.io/web"

	"xorm.io/xorm"
)

// NamespaceUser represents a namespace <-> user relation
type NamespaceUser struct {
	// The unique, numeric id of this namespace <-> user relation.
	ID int64 `xorm:"bigint autoincr not null unique pk" json:"id" param:"namespace"`
	// The username.
	Username string `xorm:"-" json:"user_id" param:"user"`
	UserID   int64  `xorm:"bigint not null INDEX" json:"-"`
	// The namespace id
	NamespaceID int64 `xorm:"bigint not null INDEX" json:"-" param:"namespace"`
	// The right this user has. 0 = Read only, 1 = Read & Write, 2 = Admin. See the docs for more details.
	Right Right `xorm:"bigint INDEX not null default 0" json:"right" valid:"length(0|2)" maximum:"2" default:"0"`

	// A timestamp when this relation was created. You cannot change this value.
	Created time.Time `xorm:"created not null" json:"created"`
	// A timestamp when this relation was last updated. You cannot change this value.
	Updated time.Time `xorm:"updated not null" json:"updated"`

	web.CRUDable `xorm:"-" json:"-"`
	web.Rights   `xorm:"-" json:"-"`
}

// TableName is the table name for NamespaceUser
func (NamespaceUser) TableName() string {
	return "users_namespaces"
}

// Create creates a new namespace <-> user relation
// @Summary Add a user to a namespace
// @Description Gives a user access to a namespace.
// @tags sharing
// @Accept json
// @Produce json
// @Security JWTKeyAuth
// @Param id path int true "Namespace ID"
// @Param namespace body models.NamespaceUser true "The user you want to add to the namespace."
// @Success 201 {object} models.NamespaceUser "The created user<->namespace relation."
// @Failure 400 {object} web.HTTPError "Invalid user namespace object provided."
// @Failure 404 {object} web.HTTPError "The user does not exist."
// @Failure 403 {object} web.HTTPError "The user does not have access to the namespace"
// @Failure 500 {object} models.Message "Internal error"
// @Router /namespaces/{id}/users [put]
func (nu *NamespaceUser) Create(s *xorm.Session, a web.Auth) (err error) {
	// Reset the id
	nu.ID = 0

	// Check if the right is valid
	if err := nu.Right.isValid(); err != nil {
		return err
	}

	// Check if the namespace exists
	n, err := GetNamespaceByID(s, nu.NamespaceID)
	if err != nil {
		return
	}

	// Check if the user exists
	user, err := user2.GetUserByUsername(s, nu.Username)
	if err != nil {
		return err
	}
	nu.UserID = user.ID

	// Check if the user already has access or is owner of that namespace
	// We explicitly DO NOT check for teams here
	if n.OwnerID == nu.UserID {
		return ErrUserAlreadyHasNamespaceAccess{UserID: nu.UserID, NamespaceID: nu.NamespaceID}
	}

	exist, err := s.
		Where("namespace_id = ? AND user_id = ?", nu.NamespaceID, nu.UserID).
		Get(&NamespaceUser{})
	if err != nil {
		return
	}
	if exist {
		return ErrUserAlreadyHasNamespaceAccess{UserID: nu.UserID, NamespaceID: nu.NamespaceID}
	}

	// Insert user <-> namespace relation
	_, err = s.Insert(nu)
	if err != nil {
		return err
	}

	return events.Dispatch(&NamespaceSharedWithUserEvent{
		Namespace: n,
		User:      user,
		Doer:      a,
	})
}

// Delete deletes a namespace <-> user relation
// @Summary Delete a user from a namespace
// @Description Delets a user from a namespace. The user won't have access to the namespace anymore.
// @tags sharing
// @Produce json
// @Security JWTKeyAuth
// @Param namespaceID path int true "Namespace ID"
// @Param userID path int true "user ID"
// @Success 200 {object} models.Message "The user was successfully deleted."
// @Failure 403 {object} web.HTTPError "The user does not have access to the namespace"
// @Failure 404 {object} web.HTTPError "user or namespace does not exist."
// @Failure 500 {object} models.Message "Internal error"
// @Router /namespaces/{namespaceID}/users/{userID} [delete]
func (nu *NamespaceUser) Delete(s *xorm.Session, a web.Auth) (err error) {

	// Check if the user exists
	user, err := user2.GetUserByUsername(s, nu.Username)
	if err != nil {
		return
	}
	nu.UserID = user.ID

	// Check if the user has access to the namespace
	has, err := s.
		Where("user_id = ? AND namespace_id = ?", nu.UserID, nu.NamespaceID).
		Get(&NamespaceUser{})
	if err != nil {
		return
	}
	if !has {
		return ErrUserDoesNotHaveAccessToNamespace{NamespaceID: nu.NamespaceID, UserID: nu.UserID}
	}

	_, err = s.
		Where("user_id = ? AND namespace_id = ?", nu.UserID, nu.NamespaceID).
		Delete(&NamespaceUser{})
	return
}

// ReadAll gets all users who have access to a namespace
// @Summary Get users on a namespace
// @Description Returns a namespace with all users which have access on a given namespace.
// @tags sharing
// @Accept json
// @Produce json
// @Param id path int true "Namespace ID"
// @Param page query int false "The page number. Used for pagination. If not provided, the first page of results is returned."
// @Param per_page query int false "The maximum number of items per page. Note this parameter is limited by the configured maximum of items per page."
// @Param s query string false "Search users by its name."
// @Security JWTKeyAuth
// @Success 200 {array} models.UserWithRight "The users with the right they have."
// @Failure 403 {object} web.HTTPError "No right to see the namespace."
// @Failure 500 {object} models.Message "Internal error"
// @Router /namespaces/{id}/users [get]
func (nu *NamespaceUser) ReadAll(s *xorm.Session, a web.Auth, search string, page int, perPage int) (result interface{}, resultCount int, numberOfTotalItems int64, err error) {
	// Check if the user has access to the namespace
	l := Namespace{ID: nu.NamespaceID}
	canRead, _, err := l.CanRead(s, a)
	if err != nil {
		return nil, 0, 0, err
	}
	if !canRead {
		return nil, 0, 0, ErrNeedToHaveNamespaceReadAccess{}
	}

	// Get all users
	all := []*UserWithRight{}
	limit, start := getLimitFromPageIndex(page, perPage)
	query := s.
		Join("INNER", "users_namespaces", "user_id = users.id").
		Where("users_namespaces.namespace_id = ?", nu.NamespaceID).
		Where(db.ILIKE("users.username", search))
	if limit > 0 {
		query = query.Limit(limit, start)
	}
	err = query.Find(&all)
	if err != nil {
		return nil, 0, 0, err
	}

	// Obfuscate all user emails
	for _, u := range all {
		u.Email = ""
	}

	numberOfTotalItems, err = s.
		Join("INNER", "users_namespaces", "user_id = users.id").
		Where("users_namespaces.namespace_id = ?", nu.NamespaceID).
		Where("users.username LIKE ?", "%"+search+"%").
		Count(&UserWithRight{})

	return all, len(all), numberOfTotalItems, err
}

// Update updates a user <-> namespace relation
// @Summary Update a user <-> namespace relation
// @Description Update a user <-> namespace relation. Mostly used to update the right that user has.
// @tags sharing
// @Accept json
// @Produce json
// @Param namespaceID path int true "Namespace ID"
// @Param userID path int true "User ID"
// @Param namespace body models.NamespaceUser true "The user you want to update."
// @Security JWTKeyAuth
// @Success 200 {object} models.NamespaceUser "The updated user <-> namespace relation."
// @Failure 403 {object} web.HTTPError "The user does not have admin-access to the namespace"
// @Failure 404 {object} web.HTTPError "User or namespace does not exist."
// @Failure 500 {object} models.Message "Internal error"
// @Router /namespaces/{namespaceID}/users/{userID} [post]
func (nu *NamespaceUser) Update(s *xorm.Session, a web.Auth) (err error) {

	// Check if the right is valid
	if err := nu.Right.isValid(); err != nil {
		return err
	}

	// Check if the user exists
	user, err := user2.GetUserByUsername(s, nu.Username)
	if err != nil {
		return err
	}
	nu.UserID = user.ID

	_, err = s.
		Where("namespace_id = ? AND user_id = ?", nu.NamespaceID, nu.UserID).
		Cols("right").
		Update(nu)
	return
}
