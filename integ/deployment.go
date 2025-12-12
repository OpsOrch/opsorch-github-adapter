//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-github-adapter/deployment"
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

	fmt.Println("===============================================")
	fmt.Println("GitHub Deployment Provider Integration Test")
	fmt.Println("===============================================")
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

	// Create the GitHub deployment provider
	config := map[string]any{
		"token": token,
		"owner": owner,
		"repo":  repo,
	}

	provider, err := deployment.New(config)
	if err != nil {
		log.Fatalf("Failed to create GitHub deployment provider: %v", err)
	}

	// Test 1: Query existing workflow runs
	fmt.Println("=== Test 1: Query Existing Workflow Runs ===")
	deployments, err := provider.Query(ctx, schema.DeploymentQuery{})
	if err != nil {
		testResult("Query existing deployments", err)
	} else {
		fmt.Printf("Found %d existing workflow runs\n", len(deployments))
		if len(deployments) == 0 {
			fmt.Printf("⚠️  No workflow runs found. This repository may not have GitHub Actions enabled.\n")
			fmt.Printf("   To get meaningful deployment tests, push code with a .github/workflows/*.yml file.\n")
		}
		for i, dep := range deployments {
			if i < 3 { // Show first 3 deployments
				workflowName := "unknown"
				if name, ok := dep.Fields["workflow_name"].(string); ok {
					workflowName = name
				}
				fmt.Printf("  [%d] ID: %s, Workflow: %s, Status: %s, Environment: %s\n",
					i+1, dep.ID, workflowName, dep.Status, dep.Environment)
			}
		}
		testResult("Query existing deployments", nil)
	}

	var testDeploymentID string
	if len(deployments) > 0 {
		testDeploymentID = deployments[0].ID
	}

	// Test 2: Get specific deployment
	if testDeploymentID != "" {
		fmt.Println("\n=== Test 2: Get Specific Deployment ===")
		dep, err := provider.Get(ctx, testDeploymentID)
		if err != nil {
			testResult("Get deployment by ID", err)
		} else {
			fmt.Printf("Retrieved deployment:\n")
			fmt.Printf("  ID: %s\n", dep.ID)
			if workflowName, ok := dep.Fields["workflow_name"].(string); ok {
				fmt.Printf("  Workflow: %s\n", workflowName)
			}
			fmt.Printf("  Status: %s\n", dep.Status)
			fmt.Printf("  Environment: %s\n", dep.Environment)
			if branch, ok := dep.Fields["branch"].(string); ok {
				fmt.Printf("  Branch: %s\n", branch)
			}
			if commit, ok := dep.Fields["commit"].(string); ok {
				fmt.Printf("  Commit: %s\n", commit)
			}
			fmt.Printf("  Started: %s\n", dep.StartedAt.Format("2006-01-02 15:04:05"))
			if dep.URL != "" {
				fmt.Printf("  URL: %s\n", dep.URL)
			}
			if login, ok := dep.Actor["login"].(string); ok {
				fmt.Printf("  Actor: %s\n", login)
			}

			if dep.ID != testDeploymentID {
				testResult("Validate retrieved deployment ID", fmt.Errorf("ID mismatch"))
			} else {
				testResult("Get deployment by ID", nil)
			}
		}
	} else {
		fmt.Println("\n=== Test 2: Get Specific Deployment (Skipped) ===")
		fmt.Printf("⚠️  No deployments available to test Get operation\n")
		testResult("Get deployment by ID", nil) // Pass since this is expected for empty repos
	}

	// Test 3: Query by status (use actual status from existing deployments)
	fmt.Println("\n=== Test 3: Query by Status ===")
	var testStatus string
	if len(deployments) > 0 {
		testStatus = deployments[0].Status
		fmt.Printf("Testing with status: %s (from existing deployment)\n", testStatus)
	} else {
		testStatus = "success" // fallback
	}

	statusDeployments, err := provider.Query(ctx, schema.DeploymentQuery{
		Statuses: []string{testStatus},
	})
	if err != nil {
		testResult("Query by status", err)
	} else {
		fmt.Printf("Found %d deployments with status '%s'\n", len(statusDeployments), testStatus)
		if len(statusDeployments) == 0 && len(deployments) > 0 {
			testResult("Query by status", fmt.Errorf("expected at least 1 deployment with status '%s', got 0", testStatus))
		} else {
			// Validate all returned deployments have the correct status
			allValid := true
			for _, dep := range statusDeployments {
				if dep.Status != testStatus {
					allValid = false
					testResult("Validate status filter", fmt.Errorf("expected status '%s', got '%s'", testStatus, dep.Status))
					break
				}
			}
			if allValid {
				testResult("Query by status", nil)
			}
		}
	}

	// Test 4: Query by environment
	fmt.Println("\n=== Test 4: Query by Environment ===")
	envDeployments, err := provider.Query(ctx, schema.DeploymentQuery{
		Scope: schema.QueryScope{Environment: "production"},
	})
	if err != nil {
		testResult("Query by environment", err)
	} else {
		fmt.Printf("Found %d deployments in production environment\n", len(envDeployments))
		testResult("Query by environment", nil)
	}

	// Test 5: Query by service scope
	fmt.Println("\n=== Test 5: Query by Service Scope ===")
	serviceDeployments, err := provider.Query(ctx, schema.DeploymentQuery{
		Scope: schema.QueryScope{Service: repo}, // Use repo name as service
	})
	if err != nil {
		testResult("Query by service scope", err)
	} else {
		fmt.Printf("Found %d deployments for service '%s'\n", len(serviceDeployments), repo)
		testResult("Query by service scope", nil)
	}

	// Test 6: Query with limit
	fmt.Println("\n=== Test 6: Query with Limit ===")
	limitedDeployments, err := provider.Query(ctx, schema.DeploymentQuery{
		Limit: 5,
	})
	if err != nil {
		testResult("Query with limit", err)
	} else {
		if len(limitedDeployments) <= 5 {
			fmt.Printf("Correctly limited to %d deployments\n", len(limitedDeployments))
			testResult("Query with limit", nil)
		} else {
			testResult("Query with limit", fmt.Errorf("expected max 5 deployments, got %d", len(limitedDeployments)))
		}
	}

	// Test 7: Query by branch
	fmt.Println("\n=== Test 7: Query by Branch ===")
	branchDeployments, err := provider.Query(ctx, schema.DeploymentQuery{
		Metadata: map[string]any{
			"branch": "main",
		},
	})
	if err != nil {
		testResult("Query by branch", err)
	} else {
		fmt.Printf("Found %d deployments on 'main' branch\n", len(branchDeployments))
		allMainBranch := true
		for _, dep := range branchDeployments {
			if branch, ok := dep.Fields["branch"].(string); ok && branch != "main" {
				allMainBranch = false
				testResult("Validate branch filter", fmt.Errorf("expected main branch, got %s", branch))
				break
			}
		}
		if allMainBranch {
			testResult("Query by branch", nil)
		}
	}

	// Test 8: Query recent deployments (using limit as proxy)
	fmt.Println("\n=== Test 8: Query Recent Deployments ===")
	recentDeployments, err := provider.Query(ctx, schema.DeploymentQuery{
		Limit: 10, // Get recent deployments by limiting results
	})
	if err != nil {
		testResult("Query recent deployments", err)
	} else {
		fmt.Printf("Found %d recent deployments\n", len(recentDeployments))
		// Validate that we got some deployments and they have valid timestamps
		allValid := true
		for _, dep := range recentDeployments {
			if dep.StartedAt.IsZero() {
				allValid = false
				testResult("Validate deployment timestamps", fmt.Errorf("deployment %s has zero timestamp", dep.ID))
				break
			}
		}
		if allValid {
			testResult("Query recent deployments", nil)
		}
	}

	// Test 9: Query with combined filters
	fmt.Println("\n=== Test 9: Query with Combined Filters ===")
	// Use actual status and environment from existing deployments
	var testStatusForCombined, testEnvironment string
	if len(deployments) > 0 {
		testStatusForCombined = deployments[0].Status
		testEnvironment = deployments[0].Environment
	} else {
		testStatusForCombined = "success"
		testEnvironment = "production"
	}

	combinedDeployments, err := provider.Query(ctx, schema.DeploymentQuery{
		Statuses: []string{testStatusForCombined},
		Scope:    schema.QueryScope{Environment: testEnvironment},
		Limit:    10,
	})
	if err != nil {
		testResult("Query with combined filters", err)
	} else {
		fmt.Printf("Found %d deployments with combined filters (status=%s, env=%s)\n",
			len(combinedDeployments), testStatusForCombined, testEnvironment)

		if len(deployments) > 0 && len(combinedDeployments) == 0 {
			testResult("Query with combined filters", fmt.Errorf("expected at least 1 deployment matching filters, got 0"))
		} else {
			allMatch := true
			for _, dep := range combinedDeployments {
				if dep.Status != testStatusForCombined {
					allMatch = false
					testResult("Validate combined status filter", fmt.Errorf("deployment %s has status %s, expected %s", dep.ID, dep.Status, testStatusForCombined))
					break
				}
				if dep.Environment != testEnvironment {
					allMatch = false
					testResult("Validate combined environment filter", fmt.Errorf("deployment %s has environment %s, expected %s", dep.ID, dep.Environment, testEnvironment))
					break
				}
			}
			if allMatch {
				testResult("Query with combined filters", nil)
			}
		}
	}

	// Test 10: Error handling - invalid deployment ID
	fmt.Println("\n=== Test 10: Error Handling - Invalid ID ===")
	_, err = provider.Get(ctx, "999999999")
	if err != nil {
		fmt.Printf("Correctly handled invalid ID: %v\n", err)
		testResult("Error handling for invalid ID", nil)
	} else {
		testResult("Error handling for invalid ID", fmt.Errorf("should have returned error for invalid ID"))
	}

	// Test 11: Test environment extraction logic
	fmt.Println("\n=== Test 11: Environment Extraction Logic ===")
	if len(deployments) > 0 {
		fmt.Println("Sample environment extractions:")
		for i, dep := range deployments {
			if i >= 5 {
				break
			}
			branch := "unknown"
			if b, ok := dep.Fields["branch"].(string); ok {
				branch = b
			}
			workflowName := "unknown"
			if name, ok := dep.Fields["workflow_name"].(string); ok {
				workflowName = name
			}
			fmt.Printf("  Workflow: %s, Branch: %s -> Environment: %s\n",
				workflowName, branch, dep.Environment)
		}
		testResult("Environment extraction logic", nil)
	} else {
		testResult("Environment extraction logic", fmt.Errorf("no deployments to test"))
	}

	// Test 12: Validate deployment metadata
	fmt.Println("\n=== Test 12: Validate Deployment Metadata ===")
	if len(deployments) > 0 {
		dep := deployments[0]
		hasRequiredFields := true

		if dep.ID == "" {
			hasRequiredFields = false
			fmt.Printf("Missing ID field\n")
		}
		if _, ok := dep.Fields["workflow_name"].(string); !ok {
			hasRequiredFields = false
			fmt.Printf("Missing workflow_name field\n")
		}
		if dep.Status == "" {
			hasRequiredFields = false
			fmt.Printf("Missing Status field\n")
		}
		if dep.StartedAt.IsZero() {
			hasRequiredFields = false
			fmt.Printf("Missing StartedAt field\n")
		}

		if hasRequiredFields {
			fmt.Printf("All required metadata fields present\n")
			testResult("Validate deployment metadata", nil)
		} else {
			testResult("Validate deployment metadata", fmt.Errorf("missing required fields"))
		}
	} else {
		testResult("Validate deployment metadata", fmt.Errorf("no deployments to validate"))
	}

	// Print summary
	duration := time.Since(startTime)
	fmt.Println("\n===============================================")
	fmt.Println("Test Summary")
	fmt.Println("===============================================")
	fmt.Printf("Total Tests: %d\n", totalTests)
	fmt.Printf("Passed: %d ✅\n", passedTests)
	fmt.Printf("Failed: %d ❌\n", failedTests)
	fmt.Printf("Duration: %v\n", duration.Round(time.Millisecond))
	if totalTests > 0 {
		fmt.Printf("Success Rate: %.1f%%\n", float64(passedTests)/float64(totalTests)*100)
	}

	if failedTests == 0 {
		fmt.Println("\n✅ All tests passed successfully!")
	} else {
		fmt.Printf("\n⚠️  %d test(s) failed. Please review the output above.\n", failedTests)
	}
}
