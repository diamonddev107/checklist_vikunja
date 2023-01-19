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
	"code.vikunja.io/web"
	"xorm.io/xorm"
)

// CanCreate checks if the user can create a new user <-> list relation
func (lu *ListUser) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	return lu.canDoListUser(s, a)
}

// CanDelete checks if the user can delete a user <-> list relation
func (lu *ListUser) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	return lu.canDoListUser(s, a)
}

// CanUpdate checks if the user can update a user <-> list relation
func (lu *ListUser) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	return lu.canDoListUser(s, a)
}

func (lu *ListUser) canDoListUser(s *xorm.Session, a web.Auth) (bool, error) {
	// Link shares aren't allowed to do anything
	if _, is := a.(*LinkSharing); is {
		return false, nil
	}

	// Get the list and check if the user has write access on it
	l := List{ID: lu.ListID}
	return l.IsAdmin(s, a)
}
