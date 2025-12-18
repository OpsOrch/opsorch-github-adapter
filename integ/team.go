package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-github-adapter/team"
)

func main() {
	// Get configuration from environment
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("GITHUB_TOKEN environment variable is required")
	}

	org := os.Getenv("GITHUB_ORG")
	if org == "" {
		org = "opsorch" // Default organization
	}

	fmt.Printf("Running GitHub team integration tests against organization: %s\n", org)

	// Create provider
	config := map[string]any{
		"token":        token,
		"organization": org,
	}

	provider, err := team.New(config)
	if err != nil {
		log.Fatalf("Failed to create GitHub team provider: %v", err)
	}

	ctx := context.Background()

	// Test 1: Query all teams
	fmt.Println("\n=== Test 1: Query all teams ===")
	teams, err := provider.Query(ctx, schema.TeamQuery{})
	if err != nil {
		log.Fatalf("Failed to query teams: %v", err)
	}

	fmt.Printf("Found %d teams:\n", len(teams))
	for _, team := range teams {
		fmt.Printf("  - ID: %s, Name: %s, Parent: %s\n", team.ID, team.Name, team.Parent)
		fmt.Printf("    Tags: %v\n", team.Tags)
	}

	if len(teams) == 0 {
		fmt.Println("No teams found. This might be expected if the organization has no teams.")
		return
	}

	// Test 2: Get specific team
	fmt.Println("\n=== Test 2: Get specific team ===")
	firstTeam := teams[0]
	team, err := provider.Get(ctx, firstTeam.ID)
	if err != nil {
		log.Fatalf("Failed to get team %s: %v", firstTeam.ID, err)
	}

	fmt.Printf("Team details:\n")
	fmt.Printf("  ID: %s\n", team.ID)
	fmt.Printf("  Name: %s\n", team.Name)
	fmt.Printf("  Parent: %s\n", team.Parent)
	fmt.Printf("  Tags: %v\n", team.Tags)
	fmt.Printf("  Metadata keys: %v\n", getKeys(team.Metadata))

	// Test 3: Get team members
	fmt.Println("\n=== Test 3: Get team members ===")
	members, err := provider.Members(ctx, firstTeam.ID)
	if err != nil {
		log.Fatalf("Failed to get team members for %s: %v", firstTeam.ID, err)
	}

	fmt.Printf("Found %d members in team %s:\n", len(members), firstTeam.Name)
	for _, member := range members {
		fmt.Printf("  - ID: %s, Name: %s, Email: %s, Role: %s\n",
			member.ID, member.Name, member.Email, member.Role)
	}

	// Test 4: Query teams by name filter
	if len(teams) > 0 {
		fmt.Println("\n=== Test 4: Query teams by name filter ===")
		searchName := firstTeam.Name
		if len(searchName) > 3 {
			searchName = searchName[:3] // Use first 3 characters for partial match
		}

		filteredTeams, err := provider.Query(ctx, schema.TeamQuery{
			Name: searchName,
		})
		if err != nil {
			log.Fatalf("Failed to query teams by name: %v", err)
		}

		fmt.Printf("Teams matching '%s': %d\n", searchName, len(filteredTeams))
		for _, team := range filteredTeams {
			fmt.Printf("  - %s (%s)\n", team.Name, team.ID)
		}
	}

	// Test 5: Query teams by tags
	fmt.Println("\n=== Test 5: Query teams by tags ===")
	taggedTeams, err := provider.Query(ctx, schema.TeamQuery{
		Tags: map[string]string{
			"provider": "github",
		},
	})
	if err != nil {
		log.Fatalf("Failed to query teams by tags: %v", err)
	}

	fmt.Printf("Teams with provider=github tag: %d\n", len(taggedTeams))

	fmt.Println("\n=== Integration tests completed successfully! ===")
}

func getKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
