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

package user

import (
	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/notifications"
	"code.vikunja.io/api/pkg/utils"
	"xorm.io/xorm"
)

// PasswordReset holds the data to reset a password
type PasswordReset struct {
	// The previously issued reset token.
	Token string `json:"token"`
	// The new password for this user.
	NewPassword string `json:"new_password"`
}

// ResetPassword resets a users password
func ResetPassword(s *xorm.Session, reset *PasswordReset) (err error) {

	// Check if the password is not empty
	if reset.NewPassword == "" {
		return ErrNoUsernamePassword{}
	}

	if reset.Token == "" {
		return ErrNoPasswordResetToken{}
	}

	// Check if we have a token
	user := &User{}
	exists, err := s.
		Where("password_reset_token = ?", reset.Token).
		Get(user)
	if err != nil {
		return
	}

	if !exists {
		return ErrInvalidPasswordResetToken{Token: reset.Token}
	}

	// Hash the password
	user.Password, err = HashPassword(reset.NewPassword)
	if err != nil {
		return
	}

	// Save it
	user.PasswordResetToken = ""
	_, err = s.
		Cols("password", "password_reset_token").
		Where("id = ?", user.ID).
		Update(user)
	if err != nil {
		return
	}

	// Dont send a mail if we're testing
	if !config.MailerEnabled.GetBool() {
		return
	}

	// Send a mail to the user to notify it his password was changed.
	n := &PasswordChangedNotification{
		User: user,
	}

	err = notifications.Notify(user, n)
	return
}

// PasswordTokenRequest defines the request format for password reset resqest
type PasswordTokenRequest struct {
	Email string `json:"email" valid:"email,length(0|250)" maxLength:"250"`
}

// RequestUserPasswordResetTokenByEmail inserts a random token to reset a users password into the databsse
func RequestUserPasswordResetTokenByEmail(s *xorm.Session, tr *PasswordTokenRequest) (err error) {
	if tr.Email == "" {
		return ErrNoUsernamePassword{}
	}

	// Check if the user exists
	user, err := GetUserWithEmail(s, &User{Email: tr.Email})
	if err != nil {
		return
	}

	return RequestUserPasswordResetToken(s, user)
}

// RequestUserPasswordResetToken sends a user a password reset email.
func RequestUserPasswordResetToken(s *xorm.Session, user *User) (err error) {
	// Generate a token and save it
	user.PasswordResetToken = utils.MakeRandomString(400)

	// Save it
	_, err = s.
		Where("id = ?", user.ID).
		Update(user)
	if err != nil {
		return
	}

	// Dont send a mail if we're testing
	if !config.MailerEnabled.GetBool() {
		return
	}

	n := &ResetPasswordNotification{
		User: user,
	}

	err = notifications.Notify(user, n)
	return
}
