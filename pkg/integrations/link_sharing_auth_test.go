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

package integrations

import (
	"net/http"
	"testing"

	"code.vikunja.io/api/pkg/models"
	apiv1 "code.vikunja.io/api/pkg/routes/api/v1"
	"github.com/stretchr/testify/assert"
)

func TestLinkSharingAuth(t *testing.T) {
	t.Run("Without Password", func(t *testing.T) {
		rec, err := newTestRequest(t, http.MethodPost, apiv1.AuthenticateLinkShare, ``, nil, map[string]string{"share": "test"})
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"token":"`)
		assert.Contains(t, rec.Body.String(), `"list_id":1`)
	})
	t.Run("Without Password, Password Provided", func(t *testing.T) {
		rec, err := newTestRequest(t, http.MethodPost, apiv1.AuthenticateLinkShare, `{"password":"somethingsomething"}`, nil, map[string]string{"share": "test"})
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"token":"`)
		assert.Contains(t, rec.Body.String(), `"list_id":1`)
	})
	t.Run("With Password, No Password Provided", func(t *testing.T) {
		_, err := newTestRequest(t, http.MethodPost, apiv1.AuthenticateLinkShare, ``, nil, map[string]string{"share": "testWithPassword"})
		assert.Error(t, err)
		assertHandlerErrorCode(t, err, models.ErrCodeLinkSharePasswordRequired)
	})
	t.Run("With Password, Password Provided", func(t *testing.T) {
		rec, err := newTestRequest(t, http.MethodPost, apiv1.AuthenticateLinkShare, `{"password":"1234"}`, nil, map[string]string{"share": "testWithPassword"})
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"token":"`)
		assert.Contains(t, rec.Body.String(), `"list_id":1`)
	})
	t.Run("With Wrong Password", func(t *testing.T) {
		_, err := newTestRequest(t, http.MethodPost, apiv1.AuthenticateLinkShare, `{"password":"someWrongPassword"}`, nil, map[string]string{"share": "testWithPassword"})
		assert.Error(t, err)
		assertHandlerErrorCode(t, err, models.ErrCodeLinkSharePasswordInvalid)
	})
}
