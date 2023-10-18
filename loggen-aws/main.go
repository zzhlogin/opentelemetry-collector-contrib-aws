package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"strings"
	"time"
)

type ChangelogEntry struct {
	ChangeType string   `yaml:"change_type"`
	Component  string   `yaml:"component"`
	Note       string   `yaml:"note"`
	Issues     []int    `yaml:"issues"`
	Subtext    string   `yaml:"subtext"`
	ChangeLogs []string `yaml:"change_logs"`
}

func main() {
	// Read the YAML file
	yamlFile, err := ioutil.ReadFile(".chloggen-aws/TEMPLATE.yaml")
	if err != nil {
		fmt.Printf("Error reading YAML file: %v\n", err)
		return
	}

	// Parse the YAML into a ChangelogEntry struct
	var entry ChangelogEntry
	err = yaml.Unmarshal(yamlFile, &entry)
	if err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		return
	}

	// Read the existing content of the CHANGELOG-AWS.md file
	existingContent, err := ioutil.ReadFile("CHANGELOG-AWS.md")
	if err != nil {
		fmt.Printf("Error reading CHANGELOG-AWS.md: %v\n", err)
		return
	}
	currentDate := time.Now().Format("2006-01-02")
	// Format the new entry
	entryText := fmt.Sprintf("## %s  -  %s\n\n", strings.Title(entry.ChangeType), currentDate)
	entryText += fmt.Sprintf("**Component**: %s\n\n", entry.Component)
	entryText += fmt.Sprintf("**Description**: %s\n\n", entry.Note)
	entryText += fmt.Sprintf("**Issues**: %s\n\n", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(entry.Issues)), ", "), "[]"))
	entryText += fmt.Sprintf("**Subtext**: %s\n\n", entry.Subtext)
	entryText += fmt.Sprintf("**Change Logs**: %s\n\n", strings.Join(entry.ChangeLogs, ", "))

	// Combine the new entry and the existing content
	newContent := []byte(entryText + string(existingContent))

	// Write the combined content to the CHANGELOG-AWS.md file
	err = ioutil.WriteFile("CHANGELOG-AWS.md", newContent, 0644)
	if err != nil {
		fmt.Printf("Error writing to CHANGELOG-AWS.md: %v\n", err)
		return
	}

	fmt.Println("Changelog entry prepended to CHANGELOG-AWS.md")
}
