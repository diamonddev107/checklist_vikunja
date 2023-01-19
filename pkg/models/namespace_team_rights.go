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

// CanCreate checks if one can create a new team <-> namespace relation
func (tn *TeamNamespace) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	n := &Namespace{ID: tn.NamespaceID}
	return n.IsAdmin(s, a)
}

// CanDelete checks if a user can remove a team from a namespace. Only namespace admins can do that.
func (tn *TeamNamespace) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	n := &Namespace{ID: tn.NamespaceID}
	return n.IsAdmin(s, a)
}

// CanUpdate checks if a user can update a team from a  Only namespace admins can do that.
func (tn *TeamNamespace) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	n := &Namespace{ID: tn.NamespaceID}
	return n.IsAdmin(s, a)
}
