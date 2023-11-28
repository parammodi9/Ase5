package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type StackOverflowPost struct {
	QuestionID int    `json:"question_id"`
	Title      string `json:"title"`
	Body       string `json:"body"`
	Answers    string `json:"answers"` // Store JSON as a string
}

type GitHubIssue struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
	// include other fields as per the JSON response
}

var (
	githubAPICalls = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_github_api_calls_total",
		Help: "Total number of API calls to GitHub",
	})
	stackoverflowAPICalls = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_stackoverflow_api_calls_total",
		Help: "Total number of API calls to StackOverflow",
	})
	dataCollected = promauto.NewCounter(prometheus.CounterOpts{
		Name: "myapp_data_collected_bytes_total",
		Help: "Total amount of data collected in bytes",
	})
)

var frameworks = []struct {
	Name             string
	StackOverflowTag string
	GitHubRepo       string
}{
	{"Prometheus", "prometheus", "prometheus/prometheus"},
	{"Selenium", "selenium", "SeleniumHQ/selenium"},
	{"OpenAI", "openai", "openai/openai-cookbook"},
	{"Docker", "docker", "docker/docker"},
	{"Milvus", "milvus", "milvus-io/milvus"},
	{"Go", "golang", "golang/go"},
}

func main() {
	db := connectDatabase()

	// Fiber App Setup
	app := fiber.New()

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Welcome to the Microservices Data Fetcher")
	})

	// GET endpoint to trigger data fetching
	app.Get("/fetch-data", func(c *fiber.Ctx) error {
		go fetchDataAndStore(db) // Fetch and store data asynchronously
		return c.SendString("Data fetching initiated")
	})

	// Run Fiber App in a Goroutine
	go func() {
		log.Fatal(app.Listen(":3000"))
	}()

	// Prometheus Metrics Server
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":9091", nil))
}

func connectDatabase() *gorm.DB {
	dsn := "host=localhost user=parammodi password=9115 dbname=stackoverflowdb port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Migrate the schema
	db.AutoMigrate(&StackOverflowPost{}, &GitHubIssue{})

	return db
}

func fetchStackOverflowData() []StackOverflowPost {
	var allPosts []StackOverflowPost

	key := "SAgLCUawC9jbo9fbMPq3fQ(("

	for _, framework := range frameworks {
		stackoverflowAPICalls.Inc()

		url := fmt.Sprintf("https://api.stackexchange.com/2.3/search/advanced?order=desc&sort=activity&tagged=%s&site=stackoverflow&filter=withbody&key=%s", framework.StackOverflowTag, key)

		resp, err := http.Get(url)
		if err != nil {
			log.Fatalf("Error making request to Stack Overflow API: %v", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error reading response body: %v", err)
		}

		dataCollected.Add(float64(len(body)))

		var result struct {
			Items []StackOverflowPost `json:"items"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			log.Fatalf("Error unmarshaling response JSON: %v", err)
		}

		allPosts = append(allPosts, result.Items...)
	}
	return allPosts
}

func fetchGitHubData() []GitHubIssue {
	var allIssues []GitHubIssue
	for _, framework := range frameworks {
		githubAPICalls.Inc()

		url := fmt.Sprintf("https://api.github.com/repos/%s/issues", framework.GitHubRepo)
		fmt.Println("Fetching URL:", url) // Log the URL being accessed

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Fatalf("Error creating request: %v", err)
		}

		req.Header.Set("Authorization", "token "+"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")
		req.Header.Set("Accept", "application/vnd.github.v3+json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Error sending request to GitHub API: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("API request failed with status code: %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatalf("Error reading response body: %v", err)
		}

		var issues []GitHubIssue
		if err := json.Unmarshal(body, &issues); err != nil {
			log.Fatalf("Error unmarshaling response JSON: %v", err)
		}

		allIssues = append(allIssues, issues...)
	}
	return allIssues
}

func storeStackOverflowPost(db *gorm.DB, post StackOverflowPost) {
	db.Create(&post)
}

func storeGitHubIssue(db *gorm.DB, issue GitHubIssue) {
	var existingIssue GitHubIssue
	result := db.First(&existingIssue, issue.ID)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		db.Create(&issue)
	} else {
		db.Model(&existingIssue).Updates(issue)
	}
}

func fetchDataAndStore(db *gorm.DB) {

	stackOverflowPosts := fetchStackOverflowData()
	for _, post := range stackOverflowPosts {
		storeStackOverflowPost(db, post)
	}

	gitHubIssues := fetchGitHubData()
	for _, issue := range gitHubIssues {
		storeGitHubIssue(db, issue)
	}
}
