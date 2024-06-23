package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Command struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Count   int    `json:"count,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
}

type Result struct {
	Name          string   `json:"name"`
	StatusCodes   []string `json:"statusCodes"`
	ResponseTimes []string `json:"responseTimes"`
	Average       string   `json:"average"`
	Failures      int      `json:"failures"`
}

func format_time(t float64) string {
	if t < 1.0 {
		return fmt.Sprintf("%.f ms", t*1000)
	} else {
		return fmt.Sprintf("%.3f s", t)
	}
}

func read_curls_from_file(filename string) ([]Command, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cmds []Command
	err = json.Unmarshal(content, &cmds)
	if err != nil {
		return nil, err
	}

	return cmds, nil
}

func run_curl_cmd(cmd Command) Result {
	total := 0.0
	successfulRequests := 0
	count := cmd.Count
	if count == 0 {
		count = 3
	}
	timeout := cmd.Timeout
	if timeout == 0 {
		timeout = 30
	}
	respTimes := make([]string, count)
	statusCodes := make([]string, count)

	for i := 0; i < count; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		out, err := exec.CommandContext(ctx, "bash", "-c", cmd.Command).Output()
		if ctx.Err() == context.DeadlineExceeded {
			respTimes[i] = fmt.Sprintf("Timeout (> %ds)", timeout)
			continue
		}
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				switch exitError.ExitCode() {
				case 7:
					respTimes[i] = "error: Failed to connect to server"
				case 52:
					respTimes[i] = "error: Failed to get a response back from server"
				default:
					respTimes[i] = "error: " + err.Error()
				}
			}

			continue
		}

		output := strings.TrimSpace(string(out))
		parts := strings.Split(output, " ")
		if len(parts) != 2 {
			respTimes[i] = "Failed to parse output of curl command"
			statusCodes[i] = "Failed to parse output of curl command"
			continue
		}

		respTimeStr, statusCode := parts[0], parts[1]
		statusCodes[i] = statusCode

		respTime, err := strconv.ParseFloat(respTimeStr, 64)
		if err != nil {
			fmt.Printf("Failed to parse output of curl to FLOAT... Error: %s\n", err)
			continue
		}

		respTimes[i] = format_time(respTime)
		total += respTime
		successfulRequests++
	}

	avg := 0.0
	if successfulRequests != 0 {
		avg = total / float64(successfulRequests)
	}
	result := Result{
		Name:          cmd.Name,
		ResponseTimes: respTimes,
		Average:       format_time(avg),
		StatusCodes:   statusCodes,
		Failures:      count - successfulRequests,
	}
	return result
}

func worker(workerId int, wg *sync.WaitGroup, jobsCh <-chan Command, results chan<- Result) {
	defer wg.Done()

	for cmd := range jobsCh {
		results <- run_curl_cmd(cmd)
		log.Println("Completed execution => ", cmd.Name)
	}

	log.Printf("Shutting down worker %d", workerId)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Please enter the file name where the curls are stored... Each curl should be separated by 2 new lines")
		return
	}

	filename := os.Args[1]

	curlCmds, err := read_curls_from_file(filename)
	if err != nil {
		fmt.Printf("Encountered error reading the file: %s", err)
		return
	}

	workers := len(curlCmds)
	if workers > 5 {
		workers = 5
	}
	var wg sync.WaitGroup
	resultsCh := make(chan Result, len(curlCmds))
	jobsCh := make(chan Command, len(curlCmds))

	log.Printf("Initializing %d workers...", workers)
	for w:=0; w < workers; w++ {
		wg.Add(1)
		go worker(w, &wg, jobsCh, resultsCh)
	}

	for _, cmd := range curlCmds {
		jobsCh <- cmd
	}
	close(jobsCh)

	wg.Wait()
	close(resultsCh)

	var results []Result

	for res := range resultsCh {
		results = append(results, res)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "    ")
	encoder.SetEscapeHTML(false)
	if err = encoder.Encode(results); err != nil {
		fmt.Println("Failed to render as JSON.. ", err)
		return
	}
}
