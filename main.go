package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/serverlessworkflow/sdk-go/v2/parser"
)

const (
	outputFile = "suggestions.json"
)

type Result struct {
	FileName string `json:"fileName"`
	JSONData string `json:"jsonData"`
	Error    string `json:"error"`
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: go run main.go <path to your Go project> <working branch> <base branch>")
		return
	}
	rootPath := os.Args[1]
	workingBranch := os.Args[2]
	baseBranch := os.Args[3]

	fmt.Println("Root path: ", rootPath)
	fmt.Println("Working branch: ", workingBranch)
	fmt.Println("Base branch: ", baseBranch)

	results := []Result{}

	changedFiles, err := getChangedFiles(rootPath, workingBranch, baseBranch)
	if err != nil {
		panic(err)
	}
	sqlFiles := filterSQLFiles(changedFiles)

	for _, file := range sqlFiles {
		isAffectingWorkflowConfig, err := isAffectingWorkflowConfig(rootPath, file)
		if err != nil {
			results = append(results, Result{
				FileName: file,
				JSONData: "",
				Error:    err.Error(),
			})
			fmt.Println(err)
			continue
		}

		if isAffectingWorkflowConfig {
			jsonStrings, err := extractJSON(rootPath, file)
			if err != nil {
				results = append(results, Result{
					FileName: file,
					JSONData: "",
					Error:    err.Error(),
				})
				fmt.Println("Failed to extract JSON data from the SQL file: ", err)
				continue
			}
			for _, jsonString := range jsonStrings {
				if err := validateServerlessJSON(jsonString); err != nil {
					results = append(results, Result{
						FileName: file,
						JSONData: jsonString,
						Error:    err.Error(),
					})
				}
			}
		}
	}

	err = saveResults(results)
	if err != nil {
		fmt.Println("Failed to save the results: ", err)
	}
}

func saveResults(results []Result) error {
	// Save the results to a file
	jsonBytes, err := json.MarshalIndent(results, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(outputFile, jsonBytes, 0644)
}

func validateServerlessJSON(jsonString string) error {
	_, err := parser.FromJSONSource([]byte(jsonString))
	return err
}

func extractJSON(rootPath, filePath string) ([]string, error) {
	// Read the content of the SQL file
	content, err := os.ReadFile(rootPath + "/" + filePath)
	if err != nil {
		return nil, err
	}

	// Regular expression to match JSON data within SQL statements
	jsonRegex := regexp.MustCompile(`'{[^']*}'`)

	// Find all matches in the SQL content
	jsonStrings := jsonRegex.FindAllString(string(content), -1)

	// Remove the single quotes from the JSON strings
	for i, jsonString := range jsonStrings {
		jsonStrings[i] = jsonString[1 : len(jsonString)-1]
	}

	return jsonStrings, nil
}

func isAffectingWorkflowConfig(rootPath, sqlFileName string) (bool, error) {
	// open file and check if it contains the word "workflow_config"
	content, err := os.ReadFile(rootPath + "/" + sqlFileName)
	if err != nil {
		fmt.Println("Failed to open the file: ", err)
		return false, err
	}

	contains := strings.Contains(string(content), "workflow_config")
	return contains, nil
}

// filterSQLFiles filters the given list of file names and returns a new list containing only the SQL files.
func filterSQLFiles(fileNames []string) []string {
	sqlFiles := []string{}
	for _, file := range fileNames {
		if strings.HasSuffix(file, ".sql") {
			sqlFiles = append(sqlFiles, file)
		}
	}
	return sqlFiles
}

// getChangedFiles returns a list of changed files between two branches in a Git repository.
// It takes the rootPath of the repository, the workingBranch, and the baseBranch as input parameters.
// It executes a Git command to get the list of changed files and returns them as a slice of strings.
// If there is an error in executing the Git command, it returns nil and the error.
func getChangedFiles(rootPath, workingBranch, baseBranch string) ([]string, error) {
	cmd := fmt.Sprintf("git -C %v diff --name-only %v...%v", rootPath, baseBranch, workingBranch)
	fmt.Println("Command: ", cmd)
	output, err := exec.Command("bash", "-c", cmd).Output()
	if err != nil {
		fmt.Println("Failed to get git diff: ", err)
		return nil, err
	}

	files := string(output)
	fileNames := strings.Split(files, "\n")
	return fileNames, nil
}
