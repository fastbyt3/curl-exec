package main

import (
	"context"
	"encoding/json"
	"fmt"
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

func run_curl_cmd(cmd Command, wg *sync.WaitGroup, ch chan<- Result) {
	defer wg.Done()

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
			return
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
	ch <- result
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

	var wg sync.WaitGroup
	ch := make(chan Result, len(curlCmds))

	for _, cmd := range curlCmds {
		wg.Add(1)
		go run_curl_cmd(cmd, &wg, ch)
	}

	wg.Wait()
	close(ch)

	var results []Result

	for res := range ch {
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
