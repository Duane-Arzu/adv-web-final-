// Filename: cmd/api/users.go
package main

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Duane-Arzu/adv-web-final.git/internal/data"
	"github.com/Duane-Arzu/adv-web-final.git/internal/validator"
)

func (a *applicationDependencies) registerUserHandler(w http.ResponseWriter, r *http.Request) {
	// Get the passed in data from the request body and store in a temporary struct
	var incomingData struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	err := a.readJSON(w, r, &incomingData)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}
	// we will add the password later after we have hashed it
	user := &data.User{
		Username:  incomingData.Username,
		Email:     incomingData.Email,
		Activated: false,
	}

	err = user.Password.Set(incomingData.Password)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
	// Perform validation for the User
	v := validator.New()

	data.ValidateUser(v, user)
	if !v.IsEmpty() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = a.userModel.Insert(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateEmail):
			v.AddError("email", "a user with this email address already exists")
			a.failedValidationResponse(w, r, v.Errors)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}
	token, err := a.tokenModel.New(user.ID, 3*24*time.Hour, data.ScopeActivation)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	data := envelope{
		"user": user,
	}
	a.background(func() {
		data := map[string]any{
			"activationToken": token.Plaintext,
			"userID":          user.ID,
		}

		err = a.mailer.Send(user.Email, "user_welcome.tmpl", data)
		if err != nil {
			a.logger.Error(err.Error())
		}
	})

	// Status code 201 resource created
	err = a.writeJSON(w, http.StatusCreated, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
}

func (a *applicationDependencies) activateUserHandler(w http.ResponseWriter, r *http.Request) {
	// Get the body from the request and store in temporary struct
	var incomingData struct {
		TokenPlaintext string `json:"token"`
	}
	err := a.readJSON(w, r, &incomingData)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}
	// Validate the data
	v := validator.New()
	data.ValidateTokenPlaintext(v, incomingData.TokenPlaintext)
	if !v.IsEmpty() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}
	// Let's check if the token provided belongs to the user
	// We will implement the GetForToken() method later
	user, err := a.userModel.GetForToken(data.ScopeActivation,
		incomingData.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			a.failedValidationResponse(w, r, v.Errors)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}
	// User provided the right token so activate them
	user.Activated = true
	err = a.userModel.Update(user)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			a.editConflictResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}
	err = a.tokenModel.DeleteAllForUser(data.ScopeActivation, user.ID)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	// Send a response
	data := envelope{
		"user": user,
	}
	err = a.writeJSON(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

func (a *applicationDependencies) listUserProfileHandler(w http.ResponseWriter, r *http.Request) {
	//get the id from the URL so that we can use it to query the comments table.
	//'uid' for userID
	id, err := a.readIDParam(r, "uid")
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	//call the GetUserProfile() function to retrieve
	user, err := a.userModel.GetByID(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	//display the user information
	data := envelope{
		"user": user,
	}

	err = a.writeJSON(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
}

func (a *applicationDependencies) getUserReviewsHandler(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL so that we can use it to query the comments table.
	//'uid' for userID
	id, err := a.readIDParam(r, "uid")
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	// Get the reviews for the user
	reviews, err := a.userModel.GetUserReviews(id)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	// Display the user information along with their reviews
	data := envelope{

		"User Reviews": reviews,
	}

	err = a.writeJSON(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
}

func (a *applicationDependencies) getUserListsHandler(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL so that we can use it to query the comments table.
	//'uid' for userID
	id, err := a.readIDParam(r, "uid")
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	// Get the reviews for the user
	lists, err := a.userModel.GetUserLists(id)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	// Display the user information along with their reviews
	data := envelope{

		"User Lists": lists,
	}

	err = a.writeJSON(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
}

func (a *applicationDependencies) passwordResetHandler(w http.ResponseWriter, r *http.Request) {
	// Read the token and new password from the request body
	var incomingData struct {
		TokenPlaintext string `json:"token"`
		NewPassword    string `json:"password"`
	}
	err := a.readJSON(w, r, &incomingData)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}

	// Validate that the new password is not empty
	if incomingData.NewPassword == "" {
		a.badRequestResponse(w, r, fmt.Errorf("new password must be provided"))
		return
	}

	// Validate the token
	v := validator.New()
	data.ValidateTokenPlaintext(v, incomingData.TokenPlaintext)
	if !v.IsEmpty() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Check if the token is valid and belongs to the user
	user, err := a.userModel.GetForToken(data.ScopePasswordReset, incomingData.TokenPlaintext)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			v.AddError("token", "invalid or expired activation token")
			a.failedValidationResponse(w, r, v.Errors)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	// Fetch the user associated with the token
	user, err = a.userModel.GetByID(user.ID)
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	err = user.Password.Set(incomingData.NewPassword)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	data.ValidateUser(v, user)
	if !v.IsEmpty() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = a.userModel.Update(user)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	// Delete all existing password reset tokens for the user
	err = a.tokenModel.DeleteAllForUser(data.ScopePasswordReset, user.ID)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	// Respond with a success message
	data := envelope{
		"message": "your password was successfully reset",
	}
	err = a.writeJSON(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
}
