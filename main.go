package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// StringData represents the stored string and its properties
type StringData struct {
	ID         string           `json:"id"`
	Value      string           `json:"value"`
	Properties StringProperties `json:"properties"`
	CreatedAt  time.Time        `json:"created_at"`
}

// StringProperties contains analyzed properties of the string
type StringProperties struct {
	Length                int            `json:"length"`
	IsPalindrome          bool           `json:"is_palindrome"`
	UniqueCharacters      int            `json:"unique_characters"`
	WordCount             int            `json:"word_count"`
	SHA256Hash            string         `json:"sha256_hash"`
	CharacterFrequencyMap map[string]int `json:"character_frequency_map"`
}

// CreateStringRequest represents the request body for creating a string
type CreateStringRequest struct {
	Value string `json:"value"`
}

// GetAllStringsResponse represents the response for getting all strings
type GetAllStringsResponse struct {
	Data           []StringData           `json:"data"`
	Count          int                    `json:"count"`
	FiltersApplied map[string]interface{} `json:"filters_applied"`
}

// NaturalLanguageResponse represents the response for natural language queries
type NaturalLanguageResponse struct {
	Data             []StringData     `json:"data"`
	Count            int              `json:"count"`
	InterpretedQuery InterpretedQuery `json:"interpreted_query"`
}

// InterpretedQuery contains the parsed natural language query
type InterpretedQuery struct {
	Original      string                 `json:"original"`
	ParsedFilters map[string]interface{} `json:"parsed_filters"`
}

// In-memory storage
var (
	storage = make(map[string]*StringData)
	mu      sync.RWMutex
)

func main() {
	app := fiber.New(fiber.Config{
		ErrorHandler: customErrorHandler,
	})

	// Middleware
	app.Use(logger.New())
	app.Use(recover.New())

	// Routes - Order matters! Specific routes before parameterized routes
	app.Post("/strings", createString)
	app.Get("/strings/filter-by-natural-language", filterByNaturalLanguage)
	app.Get("/strings", getAllStrings)
	app.Get("/strings/:string_value", getSpecificString)
	app.Delete("/strings/:string_value", deleteString)

	log.Fatal(app.Listen(":8000"))
}

// customErrorHandler handles errors consistently
func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"error": message,
	})
}

// analyzeString computes all properties of a string
func analyzeString(value string) StringProperties {
	hash := computeSHA256(value)

	return StringProperties{
		Length:                len(value),
		IsPalindrome:          isPalindrome(value),
		UniqueCharacters:      countUniqueCharacters(value),
		WordCount:             countWords(value),
		SHA256Hash:            hash,
		CharacterFrequencyMap: getCharacterFrequency(value),
	}
}

// computeSHA256 generates SHA-256 hash of a string
func computeSHA256(s string) string {
	hasher := sha256.New()
	hasher.Write([]byte(s))
	return hex.EncodeToString(hasher.Sum(nil))
}

// isPalindrome checks if string is palindrome (case-insensitive)
func isPalindrome(s string) bool {
	cleaned := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(s, ""))
	length := len(cleaned)

	for i := 0; i < length/2; i++ {
		if cleaned[i] != cleaned[length-1-i] {
			return false
		}
	}

	return true
}

// countUniqueCharacters counts distinct characters
func countUniqueCharacters(s string) int {
	charSet := make(map[rune]bool)
	for _, char := range s {
		charSet[char] = true
	}
	return len(charSet)
}

// countWords counts words separated by whitespace
func countWords(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return len(strings.Fields(s))
}

// getCharacterFrequency creates character frequency map
func getCharacterFrequency(s string) map[string]int {
	frequency := make(map[string]int)
	for _, char := range s {
		frequency[string(char)]++
	}
	return frequency
}

// createString handles POST /strings
func createString(c *fiber.Ctx) error {
	var req CreateStringRequest

	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.Value == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing 'value' field")
	}

	// Check if string already exists
	hash := computeSHA256(req.Value)

	mu.RLock()
	if _, exists := storage[req.Value]; exists {
		mu.RUnlock()
		return fiber.NewError(fiber.StatusConflict, "String already exists in the system")
	}
	mu.RUnlock()

	// Analyze string
	properties := analyzeString(req.Value)

	// Create string data
	stringData := &StringData{
		ID:         hash,
		Value:      req.Value,
		Properties: properties,
		CreatedAt:  time.Now().UTC(),
	}

	// Store
	mu.Lock()
	storage[req.Value] = stringData
	mu.Unlock()

	return c.Status(fiber.StatusCreated).JSON(stringData)
}

// getSpecificString handles GET /strings/:string_value
func getSpecificString(c *fiber.Ctx) error {
	stringValue := c.Params("string_value")

	mu.RLock()
	data, exists := storage[stringValue]
	mu.RUnlock()

	if !exists {
		// return fiber.NewError(fiber.StatusNotFound, "String does not exist in the system")
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			// "status": 404,
			"error": "String does not exist in the system",
		})
	}

	return c.JSON(data)
}

// getAllStrings handles GET /strings with filtering
func getAllStrings(c *fiber.Ctx) error {
	mu.RLock()
	defer mu.RUnlock()

	var filtered []StringData
	filtersApplied := make(map[string]interface{})

	// Parse query parameters
	isPalindromeStr := c.Query("is_palindrome")
	minLengthStr := c.Query("min_length")
	maxLengthStr := c.Query("max_length")
	wordCountStr := c.Query("word_count")
	containsChar := c.Query("contains_character")

	// Convert and validate parameters
	var isPalindrome *bool
	if isPalindromeStr != "" {
		val, err := strconv.ParseBool(isPalindromeStr)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid value for is_palindrome")
		}
		isPalindrome = &val
		filtersApplied["is_palindrome"] = val
	}

	var minLength *int
	if minLengthStr != "" {
		val, err := strconv.Atoi(minLengthStr)
		if err != nil || val < 0 {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid value for min_length")
		}
		minLength = &val
		filtersApplied["min_length"] = val
	}

	var maxLength *int
	if maxLengthStr != "" {
		val, err := strconv.Atoi(maxLengthStr)
		if err != nil || val < 0 {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid value for max_length")
		}
		maxLength = &val
		filtersApplied["max_length"] = val
	}

	var wordCount *int
	if wordCountStr != "" {
		val, err := strconv.Atoi(wordCountStr)
		if err != nil || val < 0 {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid value for word_count")
		}
		wordCount = &val
		filtersApplied["word_count"] = val
	}

	if containsChar != "" {
		if len(containsChar) != 1 {
			return fiber.NewError(fiber.StatusBadRequest, "contains_character must be a single character")
		}
		filtersApplied["contains_character"] = containsChar
	}

	// Filter strings
	for _, data := range storage {
		if matchesFilters(data, isPalindrome, minLength, maxLength, wordCount, containsChar) {
			filtered = append(filtered, *data)
		}
	}

	return c.JSON(GetAllStringsResponse{
		Data:           filtered,
		Count:          len(filtered),
		FiltersApplied: filtersApplied,
	})
}

// matchesFilters checks if a string matches all filters
func matchesFilters(data *StringData, isPalindrome *bool, minLength, maxLength, wordCount *int, containsChar string) bool {
	if isPalindrome != nil && data.Properties.IsPalindrome != *isPalindrome {
		return false
	}

	if minLength != nil && data.Properties.Length < *minLength {
		return false
	}

	if maxLength != nil && data.Properties.Length > *maxLength {
		return false
	}

	if wordCount != nil && data.Properties.WordCount != *wordCount {
		return false
	}

	if containsChar != "" && !strings.Contains(strings.ToLower(data.Value), strings.ToLower(containsChar)) {
		return false
	}

	return true
}

// filterByNaturalLanguage handles GET /strings/filter-by-natural-language
func filterByNaturalLanguage(c *fiber.Ctx) error {
	query := c.Query("query")

	if query == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing 'query' parameter")
	}

	// Parse natural language query
	filters, err := parseNaturalLanguageQuery(query)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("Unable to parse query: %s", err.Error()))
	}

	// Apply filters
	mu.RLock()
	defer mu.RUnlock()

	var filtered []StringData
	for _, data := range storage {
		if matchesNaturalFilters(data, filters) {
			filtered = append(filtered, *data)
		}
	}

	return c.JSON(NaturalLanguageResponse{
		Data:  filtered,
		Count: len(filtered),
		InterpretedQuery: InterpretedQuery{
			Original:      query,
			ParsedFilters: filters,
		},
	})
}

// parseNaturalLanguageQuery converts natural language to filters
func parseNaturalLanguageQuery(query string) (map[string]interface{}, error) {
	filters := make(map[string]interface{})
	lowerQuery := strings.ToLower(query)

	// Check for palindrome
	if strings.Contains(lowerQuery, "palindrom") {
		filters["is_palindrome"] = true
	}

	// Check for word count
	if strings.Contains(lowerQuery, "single word") {
		filters["word_count"] = 1
	} else if strings.Contains(lowerQuery, "two word") {
		filters["word_count"] = 2
	}

	// Check for length constraints
	longerThanRegex := regexp.MustCompile(`longer than (\d+)`)
	if matches := longerThanRegex.FindStringSubmatch(lowerQuery); len(matches) > 1 {
		length, _ := strconv.Atoi(matches[1])
		filters["min_length"] = length + 1
	}

	shorterThanRegex := regexp.MustCompile(`shorter than (\d+)`)
	if matches := shorterThanRegex.FindStringSubmatch(lowerQuery); len(matches) > 1 {
		length, _ := strconv.Atoi(matches[1])
		filters["max_length"] = length - 1
	}

	// Check for character containment
	containsRegex := regexp.MustCompile(`contain(?:s|ing)? (?:the )?(?:letter|character) ([a-z])`)
	if matches := containsRegex.FindStringSubmatch(lowerQuery); len(matches) > 1 {
		filters["contains_character"] = matches[1]
	}

	// Check for first vowel
	if strings.Contains(lowerQuery, "first vowel") {
		filters["contains_character"] = "a"
	}

	if len(filters) == 0 {
		return nil, fmt.Errorf("could not parse any filters from query")
	}

	return filters, nil
}

// matchesNaturalFilters checks if data matches natural language filters
func matchesNaturalFilters(data *StringData, filters map[string]interface{}) bool {
	if isPalindrome, ok := filters["is_palindrome"].(bool); ok {
		if data.Properties.IsPalindrome != isPalindrome {
			return false
		}
	}

	if wordCount, ok := filters["word_count"].(int); ok {
		if data.Properties.WordCount != wordCount {
			return false
		}
	}

	if minLength, ok := filters["min_length"].(int); ok {
		if data.Properties.Length < minLength {
			return false
		}
	}

	if maxLength, ok := filters["max_length"].(int); ok {
		if data.Properties.Length > maxLength {
			return false
		}
	}

	if containsChar, ok := filters["contains_character"].(string); ok {
		if !strings.Contains(strings.ToLower(data.Value), strings.ToLower(containsChar)) {
			return false
		}
	}

	return true
}

// deleteString handles DELETE /strings/:string_value
func deleteString(c *fiber.Ctx) error {
	stringValue := c.Params("string_value")

	mu.Lock()
	defer mu.Unlock()

	if _, exists := storage[stringValue]; !exists {
		return fiber.NewError(fiber.StatusNotFound, "String does not exist in the system")
	}

	delete(storage, stringValue)

	return c.SendStatus(fiber.StatusNoContent)
}
