package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/go-github/v28/github"
	"golang.org/x/oauth2"
)

type Report struct {
	NumFailedTests      int
	NumPassedTests      int
	NumTotalTests       int
	NumFailedTestSuites int
	NumPassedTestSuites int
	NumTotalTestSuites  int
	Success             bool
	TestResults         []*TestResult
}

type TestResult struct {
	AssertionResults []*AssertionResult
	Message          string
	FilePath         string `json:"name"`
	Status           string
	Summary          string
}

type AssertionResult struct {
	AncestorTitles  []string
	FailureMessages []string
	FullName        string
	Location        Location
	Status          string
	Title           string
}

type Location struct {
	Column int
	Line   int
}

func main() {
	// read and decode the Jest result from stdin
	var report Report
	err := json.NewDecoder(os.Stdin).Decode(&report)
	if err != nil {
		log.Fatal(err)
	}

	// nothing to do, if the tests succeeded
	if report.Success {
		return
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_SECRET")},
	)
	client := github.NewClient(oauth2.NewClient(ctx, ts))

	head := os.Getenv("GITHUB_SHA")
	repoParts := strings.Split(os.Getenv("GITHUB_REPOSITORY"), "/")
	owner := repoParts[0]
	repoName := repoParts[1]

	// find the action's checkrun
	checkName := os.Getenv("GITHUB_ACTION")
	result, _, err := client.Checks.ListCheckRunsForRef(ctx, owner, repoName, head, nil)
	if err != nil {
		log.Fatal(err)
	}

	for _, run := range result.CheckRuns {
		fmt.Println(run)
	}

	if len(result.CheckRuns) == 0 {
		log.Fatalf("Unable to find check run for action: %s", checkName)
	}
	checkRun := result.CheckRuns[0]

	// add annotations for test failures
	workspacePath := os.Getenv("GITHUB_WORKSPACE") + "/"
	var annotations []*github.CheckRunAnnotation
	for _, t := range report.TestResults {
		if t.Status == "passed" {
			continue
		}

		path := strings.TrimPrefix(t.FilePath, workspacePath)

		if len(t.AssertionResults) > 0 {
			for _, a := range t.AssertionResults {
				if a.Status == "passed" {
					continue
				}

				if len(a.FailureMessages) == 0 {
					a.FailureMessages = append(a.FailureMessages, a.FullName)
				}

				annotations = append(annotations, &github.CheckRunAnnotation{
					Path:            github.String(path),
					StartLine:       github.Int(a.Location.Line),
					EndLine:         github.Int(a.Location.Line),
					AnnotationLevel: github.String("failure"),
					Title:           github.String(a.FullName),
					Message:         github.String(strings.Join(a.FailureMessages, "\n\n")),
				})
			}
		} else {
			// usually the case for failed test suites
			annotations = append(annotations, &github.CheckRunAnnotation{
				Path:            github.String(path),
				StartLine:       github.Int(1),
				EndLine:         github.Int(1),
				AnnotationLevel: github.String("failure"),
				Title:           github.String("Test Suite Error"),
				Message:         github.String(t.Message),
			})
		}
	}

	summary := fmt.Sprintf(
		"Test Suites: %d failed, %d passed, %d total\n",
		report.NumFailedTests,
		report.NumPassedTests,
		report.NumTotalTests,
	)
	summary += fmt.Sprintf(
		"Tests: %d failed, %d passed, %d total",
		report.NumFailedTestSuites,
		report.NumPassedTestSuites,
		report.NumTotalTestSuites,
	)

	// add annotations in #50 chunks
	for i := 0; i < len(annotations); i += 50 {
		end := i + 50

		if end > len(annotations) {
			end = len(annotations)
		}

		output := &github.CheckRunOutput{
			Title:       github.String("Result"),
			Summary:     github.String(summary),
			Annotations: annotations[i:end],
		}

		_, _, err = client.Checks.UpdateCheckRun(ctx, owner, repoName, checkRun.GetID(), github.UpdateCheckRunOptions{
			Name:    checkName,
			HeadSHA: github.String(head),
			Output:  output,
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Fatal(summary)
}
