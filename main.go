package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type Command struct {
	Name    string `json:"name"`
	Command string `json:"command"`
	Count   int    `json:"count,omitempty"`
}

type Result struct {
	Name string `json:"name"`
	ResponseTimes []string `json:"responseTimes"`
	Average string `json:"average"`
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
	count := cmd.Count
	if count == 0 {
		count = 3
	}
	respTimes := make([]string, count)

	for i := 0; i < count; i++ {
		out, err := exec.Command("bash", "-c", cmd.Command).Output()
		if err != nil {
			fmt.Printf("Failed to run curl cmd: %s\nRan into err: %s", cmd.Name, err)
			return
		}

		respTimeStr := strings.TrimSpace(string(out))
		respTime, err := strconv.ParseFloat(respTimeStr, 64)
		if err != nil {
			fmt.Printf("Failed to parse output of curl to FLOAT... Error: %s", err)
			return
		}

		respTimes[i] = respTimeStr+"s"
		total += respTime
	}

	avg := total / float64(count)
	result := Result{
		Name:          cmd.Name,
		ResponseTimes: respTimes,
		Average:       fmt.Sprintf("%.3fs", avg),
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

	jsonFormatted, err := json.MarshalIndent(results, "", "    ")
	if err != nil {
		fmt.Println("Failed to format results as JSON.. Err: ", err)
		return
	}

	fmt.Println(string(jsonFormatted))
}
