package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type File struct {
	Name string `json:"file_name"`   // Name of the file
	Code string `json:"source_code"` // Source code located in the file
}

func main() {
	apiKey := flag.String("key", "", "API key for the generative AI service")
	flag.Parse()
	if *apiKey == "" {
		fmt.Println("API key is required")
		return
	}
	outputDir := *flag.String("output", "output", "Output directory for generated files")
	flag.Parse()
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(*apiKey))
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		return
	}
	modelName := "gemini-2.0-flash"
	// Create a scanner to read user input
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Enter your prompt: ")
	scanner.Scan() // Get user input
	prompt := scanner.Text()
	schema := genai.Schema{
		Type:        genai.TypeArray, // The top-level structure is an ARRAY (using string type)
		Description: "List of all of the filenames and source code in the files.",
		Items: &genai.Schema{ // Define the schema for EACH item WITHIN the array
			Type:        genai.TypeObject, // Each item is an OBJECT
			Description: "Object representing file.",
			Properties: map[string]*genai.Schema{
				"file_name": { // Define the 'name' property
					Type:        genai.TypeString,
					Description: "Name of the file: relative_path/file_name.file_extension",
				},
				"source_code": { // Define the 'description' property
					Type:        genai.TypeString,
					Description: "Source code located in the file.",
				},
			},
			Required: []string{"file_name", "source_code"}, // Correct property names
		},
	}

	// Create the model
	model := client.GenerativeModel(modelName)

	// Set the generation config with the schema for structured output
	model.GenerationConfig = genai.GenerationConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   &schema,
	}

	// Create the instruction prompt
	instructionPrompt := fmt.Sprintf("Based on the following request, generate the necessary code files:\n\n%s", prompt)

	// Send the request to the API
	resp, err := model.GenerateContent(ctx, genai.Text(instructionPrompt))
	if err != nil {
		fmt.Printf("Error generating content: %v\n", err)
		return
	}

	// Check if there's a response
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		fmt.Println("No response received")
		return
	}

	// Get and serialize the response
	responseData := resp.Candidates[0].Content.Parts[0]

	// Marshal the response to JSON for pretty printing
	prettyJSON, err := json.MarshalIndent(responseData, "", "  ")
	if err != nil {
		fmt.Printf("Error serializing response: %v\n", err)
		return
	}

	// Print the serialized response
	fmt.Println("\nAPI Response:")
	fmt.Println(string(prettyJSON))

	// Try to decode the response into our File struct if it's structured correctly
	var files []File
	jsonData := responseData.(genai.Text)

	jsonString := strings.TrimSpace(string(jsonData))
	if err := json.Unmarshal([]byte(jsonString), &files); err == nil {
		fmt.Printf("\nSuccessfully parsed %d file(s)\n", len(files))

		// Create output directory if it doesn't exist
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Printf("Error creating output directory: %v\n", err)
			return
		}

		// Write each file to the output directory
		for i, file := range files {
			// Create subdirectories if necessary
			fullPath := filepath.Join(outputDir, file.Name)
			dir := filepath.Dir(fullPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Printf("Error creating directory for %s: %v\n", file.Name, err)
				continue
			}

			// Write file
			if err := os.WriteFile(fullPath, []byte(file.Code), 0644); err != nil {
				fmt.Printf("Error writing file %s: %v\n", file.Name, err)
				continue
			}

			fmt.Printf("\nFile %d: %s written to %s\n", i+1, file.Name, fullPath)
		}

		fmt.Printf("\nAll files have been written to the '%s' directory\n", outputDir)
	}

}
