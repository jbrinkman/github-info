/*
Copyright © 2024 Joe Brinkman <joe.brinkman@improving.com>
*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// prCmd represents the pullrequest command
var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "List pull requests from a specified GitHub repository",
	Long: `The 'pr' command retrieves and lists pull requests from a specified GitHub repository.
You can filter the pull requests by author using the --author option. Multiple --author options
can be used to provide a list of author filters. The command outputs the number, title, author,
state, and URL of each pull request.`,
	Run: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("config")
		if configFile != "" {
			viper.SetConfigFile(configFile)
			if err := viper.ReadInConfig(); err != nil {
				log.Fatalf("Error reading config file: %v", err)
			}
		}

		// Bind flags to viper
		viper.BindPFlag("repo", cmd.Flags().Lookup("repo"))
		viper.BindPFlag("author", cmd.Flags().Lookup("author"))
		viper.BindPFlag("state", cmd.Flags().Lookup("state"))
		viper.BindPFlag("reviewer", cmd.Flags().Lookup("reviewer"))

		repo := viper.GetString("repo")
		if repo == "" {
			log.Fatal("The --repo flag is required")
		}

		authors := viper.GetStringSlice("author")
		state := viper.GetString("state")
		reviewers := viper.GetStringSlice("reviewer")

		// Convert authors and reviewers to lowercase for case-insensitive comparison
		for i, author := range authors {
			authors[i] = strings.ToLower(author)
		}
		for i, reviewer := range reviewers {
			reviewers[i] = strings.ToLower(reviewer)
		}

		// Split the repo into owner and repo name
		parts := strings.Split(repo, "/")
		if len(parts) != 2 {
			log.Fatal("Invalid repository format. Use 'owner/repo'")
		}
		owner, repoName := parts[0], parts[1]

		// Create a new Github client
		ctx := context.Background()
		client := github.NewClient(nil)

		// Construct the search query
		query := fmt.Sprintf("repo:%s/%s", owner, repoName)
		if state != "" && state != "all" {
			query += fmt.Sprintf(" state:%s", state)
		}
		for _, author := range authors {
			query += fmt.Sprintf(" author:%s", author)
		}
		query += " type:pr" // Ensure only pull requests are returned

		// Search pull requests
		searchOpts := &github.SearchOptions{}
		result, _, err := client.Search.Issues(ctx, query, searchOpts)
		if err != nil {
			log.Fatalf("Error searching pull requests: %v", err)
		}

		printPullRequests(ctx, client, result.Issues, owner, repoName, reviewers)
	},
}

func printPullRequests(ctx context.Context, client *github.Client, issues []github.Issue, owner, repoName string, reviewers []string) {
	fmt.Println("=====================================")
	fmt.Printf("Pull requests for %s/%s\n", owner, repoName)
	fmt.Printf("Count: %d\n", len(issues))
	fmt.Println("=====================================")
	fmt.Println()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	if len(reviewers) > 0 {
		t.AppendHeader(table.Row{"Number", "Title", "Author", "State", "Reviews", "Reviewer", "Approvals"})
	} else {
		t.AppendHeader(table.Row{"Number", "Title", "Author", "State", "Reviews", "Approvals"})
	}

	for _, issue := range issues {
		// Get the number of reviews for each pull request
		reviews, _, err := client.PullRequests.ListReviews(ctx, owner, repoName, *issue.Number, nil)
		if err != nil {
			log.Fatalf("Error fetching reviews for pull request #%d: %v", *issue.Number, err)
		}

		// Determine if the PR has been reviewed by the specified reviewers
		reviewerStatus := "[ ]"
		if len(reviewers) > 0 {
			for _, review := range reviews {
				reviewer := strings.ToLower(*review.User.Login)
				if contains(reviewers, reviewer) {
					reviewerStatus = "[X]"
					break
				}
			}
		}

		// Count the number of unique reviewers, excluding the PR author
		uniqueReviewers := make(map[string]struct{})
		prAuthor := strings.ToLower(*issue.User.Login)
		for _, review := range reviews {
			reviewer := strings.ToLower(*review.User.Login)
			if reviewer != prAuthor {
				uniqueReviewers[reviewer] = struct{}{}
			}
		}

		// Count the number of approvals
		approvals := 0
		for _, review := range reviews {
			if *review.State == "APPROVED" {
				approvals++
			}
		}

		// Determine the color for the PR number
		prNumber := fmt.Sprintf("%d", *issue.Number)
		createdAt := issue.CreatedAt
		if createdAt != nil {
			daysOld := time.Since(*createdAt).Hours() / 24
			if daysOld > 30 {
				prNumber = fmt.Sprintf("\033[31m%d\033[0m", *issue.Number) // Red
			} else if daysOld <= 1 {
				prNumber = fmt.Sprintf("\033[32m%d\033[0m", *issue.Number) // Green
			}
		}

		// Trim the title if it's longer than 25 characters
		title := *issue.Title
		if len(title) > 25 {
			title = title[:22] + "..."
		}

		if len(reviewers) > 0 {
			t.AppendRow([]interface{}{prNumber, title, *issue.User.Login, *issue.State, len(uniqueReviewers), reviewerStatus, approvals})
		} else {
			t.AppendRow([]interface{}{prNumber, title, *issue.User.Login, *issue.State, len(uniqueReviewers), approvals})
		}
	}

	t.Render()
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(prCmd)

	// Define the --repo flag
	prCmd.Flags().StringP("repo", "r", "", "The name of the Github repository (owner/repo)")
	//rCmd.MarkFlagRequired("repo")

	// Define the --author flag
	prCmd.Flags().StringArrayP("author", "A", []string{}, "Filter pull requests by author")

	// Define the --state flag
	prCmd.Flags().StringP("state", "s", "all", "Filter pull requests by state (ALL, OPEN, CLOSED)")

	// Define the --reviewer flag
	prCmd.Flags().StringArrayP("reviewer", "R", []string{}, "Highlight pull requests by reviewer")

	// Define the --config flag
	prCmd.Flags().StringP("config", "c", "", "Path to the configuration file")
}
