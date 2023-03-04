package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
)

func main() {

	var (
		userName, token, repositoryName string
		since                           int
	)

	flag.StringVar(&userName, "user", "", "User name")
	flag.StringVar(&token, "token", "", "GitHub token")
	flag.StringVar(&repositoryName, "repository", "", "Repository")
	flag.IntVar(&since, "since", 30, "Since when to fetch the data (in days)")

	flag.Parse()

	switch {
	case userName == "":
		log.Fatal("Username is required")
	case token == "":
		log.Fatal("Token is required")
	case repositoryName == "":
		log.Fatal("Repository is required")
	}

	auth := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	))

	client := github.NewClient(auth)
	created := time.Now().AddDate(0, 0, -since)
	format := "2006-01-02"
	createdQuery := ">=" + created.Format(format)

	var (
		totalRuns int
		totalJobs int
	)

	fmt.Printf("Fetching last %d days of data (created>=%s)\n", since, created.Format("2006-01-02"))

	var repo *github.Repository
	var res *github.Response
	var err error
	ctx := context.Background()

	page := 0

	repo, res, err = client.Repositories.Get(ctx, userName, repositoryName)
	if err != nil {
		log.Fatalf("Could not fetch repository: %s\n%v\n%v", repositoryName, res, err)
		return
	}

	log.Printf("Found: %s", repo.GetFullName())

	workflowRuns := []*github.WorkflowRun{}
	allUsage := time.Second * 0
	for {

		opts := &github.ListWorkflowRunsOptions{Created: createdQuery, ListOptions: github.ListOptions{Page: page, PerPage: 100}}

		var runs *github.WorkflowRuns
		log.Printf("Listing workflow runs for: %s", repositoryName)
		realOwner := userName
		// if user is a member of repository
		if userName != *repo.Owner.Login {
			realOwner = *repo.Owner.Login
		}
		runs, res, err = client.Actions.ListRepositoryWorkflowRuns(ctx, realOwner, repo.GetName(), opts)
		if err != nil {
			log.Fatal(err)
		}

		workflowRuns = append(workflowRuns, runs.WorkflowRuns...)

		if len(workflowRuns) == 0 {
			break
		}

		if res.NextPage == 0 {
			break
		}

		page = res.NextPage
	}
	totalRuns += len(workflowRuns)

	var entity string
	if userName != "" {
		entity = userName
	}
	log.Printf("Found %d workflow runs for %s/%s", len(workflowRuns), entity, repo.GetName())

	for _, run := range workflowRuns {
		log.Printf("Fetching jobs for: run ID: %d, startedAt: %s, conclusion: %s", run.GetID(), run.GetRunStartedAt().Format("2006-01-02 15:04:05"), run.GetConclusion())
		workflowJobs := []*github.WorkflowJob{}

		page := 0
		for {
			log.Printf("Fetching jobs for: %d, page %d", run.GetID(), page)
			jobs, res, err := client.Actions.ListWorkflowJobs(ctx, entity,
				repo.GetName(),
				run.GetID(),
				&github.ListWorkflowJobsOptions{Filter: "all", ListOptions: github.ListOptions{Page: page, PerPage: 100}})
			if err != nil {
				log.Fatal(err)
			}

			workflowJobs = append(workflowJobs, jobs.Jobs...)

			if len(jobs.Jobs) == 0 {
				break
			}

			if res.NextPage == 0 {
				break
			}
			page = res.NextPage
		}

		totalJobs += len(workflowJobs)
		log.Printf("%d jobs for workflow run: %d", len(workflowJobs), run.GetID())
		for _, job := range workflowJobs {

			if job.GetCompletedAt().Time.Unix() > job.GetStartedAt().Time.Unix() {
				dur := job.GetCompletedAt().Time.Sub(job.GetStartedAt().Time)

				allUsage += dur
				log.Printf("Job: %d [%s - %s] (%s): %s",
					job.GetID(), job.GetStartedAt().Format("2006-01-02 15:04:05"), job.GetCompletedAt().Format("2006-01-02 15:04:05"), dur.String(), job.GetConclusion())
			}
		}
	}

	fmt.Printf("Total workflow runs: %d\n", totalRuns)
	fmt.Printf("Total workflow jobs: %d\n", totalJobs)
	mins := fmt.Sprintf("%.0f mins", allUsage.Minutes())
	fmt.Printf("Total usage: %s (%s)\n", allUsage.String(), mins)
}
