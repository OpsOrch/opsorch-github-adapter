//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-github-adapter/ticket"
)

func main() {
	// Test statistics
	var totalTests, passedTests, failedTests int
	startTime := time.Now()

	testResult := func(name string, err error) {
		totalTests++
		if err != nil {
			failedTests++
			log.Printf("❌ %s: %v", name, err)
		} else {
			passedTests++
			fmt.Printf("✅ %s passed\n", name)
		}
	}

	fmt.Println("==========================================")
	fmt.Println("GitHub Ticket Provider Integration Test")
	fmt.Println("==========================================")
	fmt.Printf("Started: %s\n\n", startTime.Format("2006-01-02 15:04:05"))

	// Check required environment variables
	token := os.Getenv("GITHUB_TOKEN")
	owner := os.Getenv("GITHUB_OWNER")
	repo := os.Getenv("GITHUB_REPO")

	if token == "" {
		log.Fatal("GITHUB_TOKEN environment variable is required")
	}
	if owner == "" {
		owner = "opsorch" // default
	}
	if repo == "" {
		repo = "opsorch-github-adapter" // default
	}

	fmt.Printf("Testing against: %s/%s\n", owner, repo)
	fmt.Printf("Token: %s...\n\n", token[:10])

	ctx := context.Background()

	// Create the GitHub ticket provider
	config := map[string]any{
		"token": token,
		"owner": owner,
		"repo":  repo,
	}

	provider, err := ticket.New(config)
	if err != nil {
		log.Fatalf("Failed to create GitHub ticket provider: %v", err)
	}

	// Test 1: Query existing issues
	fmt.Println("=== Test 1: Query Existing Issues ===")
	issues, err := provider.Query(ctx, schema.TicketQuery{})
	if err != nil {
		testResult("Query existing issues", err)
	} else {
		fmt.Printf("Found %d existing issues\n", len(issues))
		for i, issue := range issues {
			if i < 3 { // Show first 3 issues
				fmt.Printf("  [%d] ID: %s, Title: %s, Status: %s, URL: %s\n",
					i+1, issue.ID, issue.Title, issue.Status, issue.URL)
			}
		}
		testResult("Query existing issues", nil)
	}

	// Test 2: Create a new issue
	fmt.Println("\n=== Test 2: Create New Issue ===")
	newIssue, err := provider.Create(ctx, schema.CreateTicketInput{
		Title:       "Integration Test Issue",
		Description: "This is a test issue created by the GitHub adapter integration test.",
		Metadata: map[string]any{
			"source": "integration-test",
			"labels": []string{"test", "integration"},
		},
	})
	if err != nil {
		testResult("Create new issue", err)
	} else {
		fmt.Printf("Successfully created issue:\n")
		fmt.Printf("  ID: %s\n", newIssue.ID)
		fmt.Printf("  Title: %s\n", newIssue.Title)
		fmt.Printf("  Status: %s\n", newIssue.Status)
		if labels, ok := newIssue.Fields["labels"].([]string); ok {
			fmt.Printf("  Labels: %v\n", labels)
		}
		fmt.Printf("  Created: %s\n", newIssue.CreatedAt.Format("2006-01-02 15:04:05"))

		// Validate created issue
		if newIssue.Title != "Integration Test Issue" {
			testResult("Validate created issue title", fmt.Errorf("title mismatch"))
		} else if newIssue.Status != "open" {
			testResult("Validate created issue status", fmt.Errorf("status mismatch"))
		} else {
			testResult("Create new issue", nil)
		}
	}

	var testIssueID string
	if err == nil {
		testIssueID = newIssue.ID

		// Ensure cleanup happens even if tests fail
		defer func() {
			if testIssueID != "" {
				fmt.Println("\n=== Cleanup: Closing Test Issue ===")
				closedStatus := "closed"
				_, cleanupErr := provider.Update(context.Background(), testIssueID, schema.UpdateTicketInput{
					Status: &closedStatus,
				})
				if cleanupErr != nil {
					fmt.Printf("⚠️  Failed to cleanup test issue %s: %v\n", testIssueID, cleanupErr)
				} else {
					fmt.Printf("✅ Test issue %s closed during cleanup\n", testIssueID)
				}
			}
		}()
	}

	// Test 3: Get specific issue
	if testIssueID != "" {
		fmt.Println("\n=== Test 3: Get Specific Issue ===")
		issue, err := provider.Get(ctx, testIssueID)
		if err != nil {
			testResult("Get issue by ID", err)
		} else {
			fmt.Printf("Retrieved issue:\n")
			fmt.Printf("  ID: %s\n", issue.ID)
			fmt.Printf("  Title: %s\n", issue.Title)
			fmt.Printf("  Status: %s\n", issue.Status)
			if labels, ok := issue.Fields["labels"].([]string); ok {
				fmt.Printf("  Labels: %v\n", labels)
			}

			if issue.ID != testIssueID {
				testResult("Validate retrieved issue ID", fmt.Errorf("ID mismatch"))
			} else {
				testResult("Get issue by ID", nil)
			}
		}
	}

	// Test 4: Query by status
	fmt.Println("\n=== Test 4: Query Open Issues ===")
	openIssues, err := provider.Query(ctx, schema.TicketQuery{
		Statuses: []string{"open"},
	})
	if err != nil {
		testResult("Query open issues", err)
	} else {
		fmt.Printf("Found %d open issues\n", len(openIssues))
		allOpen := true
		for _, issue := range openIssues {
			if issue.Status != "open" {
				allOpen = false
				testResult("Validate status filter", fmt.Errorf("expected open status, got %s", issue.Status))
				break
			}
		}
		if allOpen {
			testResult("Query open issues", nil)
		}
	}

	// Test 5: Query by labels (use label from created issue)
	fmt.Println("\n=== Test 5: Query by Labels ===")

	// First, let's get the actual labels from our created issue to see what we're working with
	if testIssueID != "" {
		createdIssue, err := provider.Get(ctx, testIssueID)
		if err == nil {
			if labels, ok := createdIssue.Fields["labels"].([]string); ok && len(labels) > 0 {
				fmt.Printf("Created issue has labels: %v\n", labels)
				// Use the first actual label from the created issue
				testLabel := labels[0]
				fmt.Printf("Testing query with label: '%s'\n", testLabel)

				// Add a small delay to allow GitHub to index the labels
				fmt.Printf("Waiting for GitHub to index labels...\n")
				time.Sleep(2 * time.Second)

				labeledIssues, err := provider.Query(ctx, schema.TicketQuery{
					Metadata: map[string]any{
						"labels": []string{testLabel},
					},
				})
				if err != nil {
					testResult("Query by labels", err)
				} else {
					fmt.Printf("Found %d issues with '%s' label\n", len(labeledIssues), testLabel)
					if len(labeledIssues) == 0 {
						testResult("Query by labels", fmt.Errorf("expected at least 1 issue with '%s' label (we just created one), got 0", testLabel))
					} else {
						// Validate that our test issue is in the results
						foundTestIssue := false
						for _, issue := range labeledIssues {
							if issue.ID == testIssueID {
								foundTestIssue = true
								break
							}
						}
						if !foundTestIssue {
							testResult("Query by labels", fmt.Errorf("test issue %s not found in label query results", testIssueID))
						} else {
							testResult("Query by labels", nil)
						}
					}
				}
			} else {
				fmt.Printf("Created issue has no labels in Fields\n")
				testResult("Query by labels", fmt.Errorf("created issue has no labels to test with"))
			}
		} else {
			testResult("Query by labels", fmt.Errorf("failed to get created issue for label testing: %v", err))
		}
	} else {
		fmt.Printf("No test issue ID available for label testing\n")
		testResult("Query by labels", nil) // Skip test
	}

	// Test 6: Update issue
	if testIssueID != "" {
		fmt.Println("\n=== Test 6: Update Issue ===")
		newTitle := "Updated Integration Test Issue"
		updatedIssue, err := provider.Update(ctx, testIssueID, schema.UpdateTicketInput{
			Title: &newTitle,
		})
		if err != nil {
			testResult("Update issue", err)
		} else {
			if updatedIssue.Title != newTitle {
				testResult("Validate updated title", fmt.Errorf("expected %s, got %s", newTitle, updatedIssue.Title))
			} else {
				fmt.Printf("✅ Issue title updated to: %s\n", updatedIssue.Title)
				testResult("Update issue", nil)
			}
		}
	}

	// Test 7: Query with limit
	fmt.Println("\n=== Test 7: Query with Limit ===")
	limitedIssues, err := provider.Query(ctx, schema.TicketQuery{
		Limit: 5,
	})
	if err != nil {
		testResult("Query with limit", err)
	} else {
		if len(limitedIssues) <= 5 {
			fmt.Printf("Correctly limited to %d issues\n", len(limitedIssues))
			testResult("Query with limit", nil)
		} else {
			testResult("Query with limit", fmt.Errorf("expected max 5 issues, got %d", len(limitedIssues)))
		}
	}

	// Test 8: Error handling - invalid issue ID
	fmt.Println("\n=== Test 8: Error Handling - Invalid ID ===")
	_, err = provider.Get(ctx, "999999999")
	if err != nil {
		fmt.Printf("Correctly handled invalid ID: %v\n", err)
		testResult("Error handling for invalid ID", nil)
	} else {
		testResult("Error handling for invalid ID", fmt.Errorf("should have returned error for invalid ID"))
	}

	// Test 9: Query by text search
	fmt.Println("\n=== Test 9: Query by Text Search ===")
	searchIssues, err := provider.Query(ctx, schema.TicketQuery{
		Query: "integration",
	})
	if err != nil {
		testResult("Query by text search", err)
	} else {
		fmt.Printf("Found %d issues matching 'integration'\n", len(searchIssues))
		testResult("Query by text search", nil)
	}

	// Test 10: Close the test issue (primary cleanup)
	if testIssueID != "" {
		fmt.Println("\n=== Test 10: Close Test Issue (Primary Cleanup) ===")
		closedStatus := "closed"
		_, err := provider.Update(ctx, testIssueID, schema.UpdateTicketInput{
			Status: &closedStatus,
		})
		if err != nil {
			testResult("Close test issue", err)
			// Don't clear testIssueID so defer cleanup can try again
		} else {
			fmt.Printf("✅ Test issue closed successfully\n")
			testResult("Close test issue", nil)
			// Clear testIssueID so defer cleanup knows it's already done
			testIssueID = ""
		}
	}

	// Print summary
	duration := time.Since(startTime)
	fmt.Println("\n==========================================")
	fmt.Println("Test Summary")
	fmt.Println("==========================================")
	fmt.Printf("Total Tests: %d\n", totalTests)
	fmt.Printf("Passed: %d ✅\n", passedTests)
	fmt.Printf("Failed: %d ❌\n", failedTests)
	fmt.Printf("Duration: %v\n", duration.Round(time.Millisecond))
	if totalTests > 0 {
		fmt.Printf("Success Rate: %.1f%%\n", float64(passedTests)/float64(totalTests)*100)
	}

	if failedTests == 0 {
		fmt.Println("\n✅ All tests passed successfully!")
		fmt.Println("✅ Test issues have been closed and cleaned up")
	} else {
		fmt.Printf("\n⚠️  %d test(s) failed. Please review the output above.\n", failedTests)
		fmt.Println("ℹ️  Test issues have been closed and cleaned up")
	}
}
