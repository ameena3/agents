// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// GitHubUser represents a user from the GitHub /user endpoint
type GitHubUser struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// GitHubPullRequest represents a pull request from the GitHub API
type GitHubPullRequest struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
}

// GetGitHubUser retrieves the authenticated user's profile from the GitHub API.
func GetGitHubUser(token string) (*GitHubUser, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()
	var user GitHubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetGitHubPullRequests retrieves open pull requests for the given owner/repo.
// Returns up to 5 results.
func GetGitHubPullRequests(owner, repo, token string) ([]GitHubPullRequest, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "AgentsSDKDemo")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github pull requests request failed: %w", err)
	}
	defer resp.Body.Close()
	var prs []GitHubPullRequest
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, err
	}
	// Return at most 5 results
	if len(prs) > 5 {
		prs = prs[:5]
	}
	return prs, nil
}
