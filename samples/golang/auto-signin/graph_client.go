// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// GraphUser represents a user from the Microsoft Graph /me endpoint
type GraphUser struct {
	DisplayName string `json:"displayName"`
	Mail        string `json:"mail"`
	ID          string `json:"id"`
}

// GetGraphUser retrieves the signed-in user's profile from Microsoft Graph using the provided Bearer token.
func GetGraphUser(token string) (*GraphUser, error) {
	req, _ := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("graph request failed: %w", err)
	}
	defer resp.Body.Close()
	var user GraphUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}
