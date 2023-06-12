package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type ProxyResponse struct {
	Success int             `json:"success"`
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    [][]interface{} `json:"data"`
}

type ProxyJob struct {
	ProxyURL    string
	ProxySchema string
}

// Color codes for success messages
const (
	SuccessColor = "\033[1;32m%s\033[0m"
)

func saveProxyToFile(proxyURL string, proxySchema string, targetURL string) {
	targetURL = strings.ReplaceAll(targetURL, "://", "_")
	targetURL = strings.ReplaceAll(targetURL, "/", "_")

	filename := fmt.Sprintf("%s.txt", targetURL)
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	_, err = file.WriteString(proxySchema + "://" + proxyURL + "\n")
	if err != nil {
		return
	}

	fmt.Printf(SuccessColor, "[SAVED] ")
	fmt.Printf("Proxy for %s saved to file: %s\n", targetURL, filename)
}

func checkProxyReachability(proxyURL string, targetURL string) bool {
	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		return false
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURLParsed),
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	resp, err := client.Get(targetURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		fmt.Printf(SuccessColor, "[Live Proxy] ")
		fmt.Println(proxyURL, targetURL)
		return true
	}

	return false
}

func checkProxy(proxyURL string, proxySchema string, wg *sync.WaitGroup, mutex *sync.Mutex, targetURLs []string) {
	defer wg.Done()

	for _, targetURL := range targetURLs {
		if checkProxyReachability(proxyURL, targetURL) {
			mutex.Lock()
			saveProxyToFile(proxyURL, proxySchema, targetURL)
			mutex.Unlock()
		}
	}
}

func worker(jobs <-chan ProxyJob, wg *sync.WaitGroup, mutex *sync.Mutex, targetURLs []string) {
	defer wg.Done()

	for job := range jobs {
		proxyURL := job.ProxyURL
		proxySchema := job.ProxySchema

		checkProxy(proxyURL, proxySchema, wg, mutex, targetURLs)
	}
}

func checkAndSaveProxies(proxyList [][]interface{}, targetURLs []string) {
	numWorkers := 100
	jobs := make(chan ProxyJob, len(proxyList))
	wg := sync.WaitGroup{}
	mutex := sync.Mutex{}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(jobs, &wg, &mutex, targetURLs)
	}

	for _, proxy := range proxyList {
		ip := proxy[0].(string)
		port := strconv.FormatFloat(proxy[1].(float64), 'f', -1, 64)
		proxySchema := strings.ToLower(proxy[2].(string))
		proxyURL := fmt.Sprintf("%s://%s:%s", proxySchema, ip, port)

		jobs <- ProxyJob{
			ProxyURL:    proxyURL,
			ProxySchema: proxySchema,
		}
	}

	close(jobs)

	wg.Wait()
}

func scrapeProxies(endpoint string) ([][]interface{}, error) {
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var proxyResp ProxyResponse
	err = json.Unmarshal(body, &proxyResp)
	if err != nil {
		return nil, err
	}

	if proxyResp.Success == 1 {
		return proxyResp.Data, nil
	} else {
		return nil, fmt.Errorf("[FAILED] [RESTART] %s", proxyResp.Message)
	}
}

func saveAllProxiesToFile(proxyList [][]interface{}) {
	file, err := os.OpenFile("allproxy.txt", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()

	for _, proxy := range proxyList {
		ip := proxy[0].(string)
		port := strconv.FormatFloat(proxy[1].(float64), 'f', -1, 64)
		proxySchema := strings.ToLower(proxy[2].(string))
		proxyURL := fmt.Sprintf("%s://%s:%s", proxySchema, ip, port)

		_, err = file.WriteString(proxyURL + "\n")
		if err != nil {
			return
		}
	}

	fmt.Printf(SuccessColor, "[SAVED] ")
	fmt.Println("allproxy.txt")
}

func main() {
	endpoint := "https://www.ditatompel.com/api/proxy/country/bd"

	proxyList, err := scrapeProxies(endpoint)
	if err != nil {
		fmt.Println(err)
		return
	}

	saveAllProxiesToFile(proxyList)

	targetURLs := []string{
		"http://circleftp.net/",
		"http://172.16.50.4",
		"http://15.1.1.4",
		"http://ftp4.circleftp.net",
		"http://172.16.50.5",
		"http://103.153.175.254/NAS1",
		"http://discoveryftp.net",
	}

	checkAndSaveProxies(proxyList, targetURLs)
}
