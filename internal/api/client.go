package api

import (
	"bytes"
	"encoding/json" // Added for dynamic parsing
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv" // Added for string to int conversion
	"strings" // Added for joining params
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/auth/credentials"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"                   // Added for requests.Integer
	devops_rdc "github.com/aliyun/alibaba-cloud-sdk-go/services/devops-rdc" // Changed import path
)

// Pipeline represents a pipeline in Aliyun DevOps.
type Pipeline struct {
	PipelineID    string    `json:"pipelineId"`
	Name          string    `json:"name"`
	Status        string    `json:"status"`        // e.g., RUNNING, SUCCESS, FAILED, CANCELED
	LastRunStatus string    `json:"lastRunStatus"` // Status of the most recent run
	LastRunTime   time.Time `json:"lastRunTime"`   // Time of the most recent run
	Creator       string    `json:"creator"`
	CreatorName   string    `json:"creatorName"` // Creator display name
	Modifier      string    `json:"modifier"`
	CreateTime    time.Time `json:"createTime"`
	UpdateTime    time.Time `json:"updateTime"`
}

// PipelineRun represents a single execution of a pipeline.
type PipelineRun struct {
	RunID       string    `json:"runId"`
	PipelineID  string    `json:"pipelineId"`
	Status      string    `json:"status"` // e.g., RUNNING, SUCCESS, FAILED, CANCELED
	StartTime   time.Time `json:"startTime"`
	FinishTime  time.Time `json:"finishTime"`
	TriggerMode string    `json:"triggerMode"` // e.g., MANUAL, PUSH, SCHEDULE
}

// PipelineGroup represents a group of pipelines.
type PipelineGroup struct {
	GroupID string `json:"groupId"`
	Name    string `json:"name"`
}

// JobAction represents an action available for a job
type JobAction struct {
	Type        string                 `json:"type"`
	DisplayType string                 `json:"displayType"`
	Data        string                 `json:"data"`
	Disable     bool                   `json:"disable"`
	Params      map[string]interface{} `json:"params"`
	Name        string                 `json:"name"`
	Title       string                 `json:"title"`
	Order       interface{}            `json:"order"`
}

// Job represents a job within a pipeline run stage
type Job struct {
	ID        int64       `json:"id"`
	JobSign   string      `json:"jobSign"`
	Name      string      `json:"name"`
	Status    string      `json:"status"`
	StartTime time.Time   `json:"startTime"`
	EndTime   time.Time   `json:"endTime"`
	Actions   []JobAction `json:"actions"`
	Result    string      `json:"result"`
}

// Stage represents a stage in a pipeline run
type Stage struct {
	Index string `json:"index"`
	Name  string `json:"name"`
	Jobs  []Job  `json:"jobs"`
}

// PipelineRunDetails represents detailed information about a pipeline run
type PipelineRunDetails struct {
	PipelineRunID int64   `json:"pipelineRunId"`
	PipelineID    int64   `json:"pipelineId"`
	Status        string  `json:"status"`
	TriggerMode   int     `json:"triggerMode"`
	CreateTime    int64   `json:"createTime"`
	UpdateTime    int64   `json:"updateTime"`
	Stages        []Stage `json:"stages"`
}

// VMDeployMachine represents a machine in a VM deployment order
type VMDeployMachine struct {
	IP           string `json:"ip"`
	MachineSn    string `json:"machineSn"`
	Status       string `json:"status"`
	ClientStatus string `json:"clientStatus"`
	BatchNum     int    `json:"batchNum"`
	CreateTime   int64  `json:"createTime"`
	UpdateTime   int64  `json:"updateTime"`
}

// VMDeployMachineInfo represents machine deployment information
type VMDeployMachineInfo struct {
	BatchNum       int               `json:"batchNum"`
	HostGroupId    int               `json:"hostGroupId"`
	DeployMachines []VMDeployMachine `json:"deployMachines"`
}

// VMDeployOrder represents a VM deployment order
type VMDeployOrder struct {
	DeployOrderId     int                 `json:"deployOrderId"`
	Status            string              `json:"status"`
	Creator           string              `json:"creator"`
	CreateTime        int64               `json:"createTime"`
	UpdateTime        int64               `json:"updateTime"`
	CurrentBatch      int                 `json:"currentBatch"`
	TotalBatch        int                 `json:"totalBatch"`
	DeployMachineInfo VMDeployMachineInfo `json:"deployMachineInfo"`
}

// VMDeployMachineLog represents deployment log for a specific machine
type VMDeployMachineLog struct {
	AliyunRegion    string `json:"aliyunRegion"`
	DeployBeginTime string `json:"deployBeginTime"`
	DeployEndTime   string `json:"deployEndTime"`
	DeployLog       string `json:"deployLog"`
	DeployLogPath   string `json:"deployLogPath"`
}

// Client is a client for interacting with the Aliyun DevOps API.
type Client struct {
	sdkClient           *devops_rdc.Client // Changed to devops_rdc
	httpClient          *http.Client       // For personal access token requests
	endpoint            string             // API endpoint for token-based requests
	personalAccessToken string             // Personal access token
	useToken            bool               // Whether to use token-based authentication
}

var debugLogger *log.Logger

func init() {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		fmt.Printf("Warning: failed to create logs directory: %v\n", err)
	}

	// Create or open log file
	logFile, err := os.OpenFile("logs/api_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("Warning: failed to open log file: %v\n", err)
		debugLogger = log.New(os.Stdout, "[DEBUG] ", log.LstdFlags)
	} else {
		debugLogger = log.New(logFile, "[DEBUG] ", log.LstdFlags)
	}
}

// createHTTPClientWithProxy creates an HTTP client with proxy support
// It reads http_proxy and https_proxy environment variables
func createHTTPClientWithProxy() *http.Client {
	transport := &http.Transport{}

	// Check for HTTP proxy
	if httpProxy := os.Getenv("http_proxy"); httpProxy != "" {
		if proxyURL, err := url.Parse(httpProxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
			if os.Getenv("FLOWT_DEBUG") == "1" {
				debugLogger.Printf("Using HTTP proxy: %s", httpProxy)
			}
		} else {
			fmt.Printf("Warning: invalid http_proxy URL: %s, error: %v\n", httpProxy, err)
		}
	}

	// Check for HTTPS proxy (takes precedence for HTTPS requests)
	if httpsProxy := os.Getenv("https_proxy"); httpsProxy != "" {
		if proxyURL, err := url.Parse(httpsProxy); err == nil {
			// For HTTPS proxy, we need to set up a custom proxy function
			// that uses different proxies for HTTP and HTTPS
			originalProxy := transport.Proxy
			transport.Proxy = func(req *http.Request) (*url.URL, error) {
				if req.URL.Scheme == "https" {
					return proxyURL, nil
				}
				if originalProxy != nil {
					return originalProxy(req)
				}
				return nil, nil
			}
			if os.Getenv("FLOWT_DEBUG") == "1" {
				debugLogger.Printf("Using HTTPS proxy: %s", httpsProxy)
			}
		} else {
			fmt.Printf("Warning: invalid https_proxy URL: %s, error: %v\n", httpsProxy, err)
		}
	}

	// If no proxy environment variables are set, use the default proxy from environment
	if transport.Proxy == nil {
		transport.Proxy = http.ProxyFromEnvironment
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

// NewClient creates a new Aliyun DevOps API client using AccessKey authentication.
// If regionId is empty, "cn-hangzhou" will be used.
// The client automatically supports http_proxy and https_proxy environment variables.
func NewClient(accessKeyId, accessKeySecret, regionId string) (*Client, error) {
	if regionId == "" {
		regionId = "cn-hangzhou" // Default region
	}

	credential := credentials.NewAccessKeyCredential(accessKeyId, accessKeySecret)

	// Create SDK client
	sdkClient, err := devops_rdc.NewClientWithOptions(regionId, sdk.NewConfig(), credential)
	if err != nil {
		return nil, fmt.Errorf("failed to create devops-rdc client: %w", err)
	}

	// Configure proxy settings from environment variables
	if httpProxy := os.Getenv("http_proxy"); httpProxy != "" {
		sdkClient.SetHttpProxy(httpProxy)
		if os.Getenv("FLOWT_DEBUG") == "1" {
			debugLogger.Printf("SDK using HTTP proxy: %s", httpProxy)
		}
	}

	if httpsProxy := os.Getenv("https_proxy"); httpsProxy != "" {
		sdkClient.SetHttpsProxy(httpsProxy)
		if os.Getenv("FLOWT_DEBUG") == "1" {
			debugLogger.Printf("SDK using HTTPS proxy: %s", httpsProxy)
		}
	}

	return &Client{
		sdkClient: sdkClient,
		useToken:  false,
	}, nil
}

// NewClientWithToken creates a new Aliyun DevOps API client using Personal Access Token authentication.
// This is the recommended authentication method according to Aliyun DevOps documentation.
// The client automatically supports http_proxy and https_proxy environment variables.
func NewClientWithToken(endpoint, personalAccessToken string) (*Client, error) {
	if endpoint == "" {
		endpoint = "openapi-rdc.aliyuncs.com" // Default endpoint from documentation
	}

	if personalAccessToken == "" {
		return nil, fmt.Errorf("personalAccessToken is required")
	}

	// For personal access token authentication, we use HTTP client directly
	// as the Aliyun SDK's BearerTokenCredential may not work properly with DevOps API
	// Create HTTP client with proxy support
	httpClient := createHTTPClientWithProxy()

	return &Client{
		httpClient:          httpClient,
		endpoint:            endpoint,
		personalAccessToken: personalAccessToken,
		useToken:            true,
	}, nil
}

// ListPipelines retrieves a list of pipelines for a given organization.
func (c *Client) ListPipelines(organizationId string) ([]Pipeline, error) {
	return c.ListPipelinesWithStatus(organizationId, nil)
}

// ListPipelinesWithStatus lists pipelines with optional status filtering
// statusList can be nil for all pipelines, or contain statuses like "RUNNING", "WAITING", etc.
func (c *Client) ListPipelinesWithStatus(organizationId string, statusList []string) ([]Pipeline, error) {
	if organizationId == "" {
		return nil, fmt.Errorf("organizationId is required for ListPipelines")
	}

	// Use different methods based on authentication type
	if c.useToken {
		return c.listPipelinesWithTokenAndStatus(organizationId, statusList)
	}

	// Use SDK for AccessKey authentication
	request := devops_rdc.CreateListPipelinesRequest() // Changed to devops_rdc
	request.Scheme = "https"                           // Usually HTTPS
	request.OrgId = organizationId                     // Assuming OrgId based on typical SDK patterns for devops-rdc

	// TODO: Add pagination handling if the API supports it.
	// request.NextToken / request.MaxResults might be relevant for pagination.

	response, err := c.sdkClient.ListPipelines(request)
	if err != nil {
		return nil, fmt.Errorf("failed to list pipelines: %w", err)
	}

	var pipelines []Pipeline
	// Assuming response.Pipelines is a slice of the SDK's pipeline type.
	// The actual field name might be different, e.g., `response.Data.Pipelines` or `response.Object`.
	// This needs verification against the actual SDK structure.
	// For now, we'll try to access `response.Pipelines` directly.
	// If `response.Pipelines` is nil or not the correct path, this will fail at runtime or not map data.

	// A common structure for Aliyun SDK responses is a `RequestId` field and then a data field.
	// Let's assume the list of pipelines is in `response.Pipelines`.
	// The type of `p` here is `*devops_rdc.ListPipelinesPipelines`.
	// We need to check the fields of this struct.
	// Example: p.PipelineId (int64), p.Name (string), p.Status (string),
	// p.Creator (string), p.Modifier (string), p.GmtCreate (string), p.GmtModified (string)

	// According to the SDK source for list_pipelines.go:
	// type ListPipelinesResponse struct {
	//         *responses.BaseResponse
	//         Success      bool                   `json:"Success" xml:"Success"`
	//         ErrorCode    string                 `json:"ErrorCode" xml:"ErrorCode"`
	//         ErrorMessage string                 `json:"ErrorMessage" xml:"ErrorMessage"`
	//         Object       map[string]interface{} `json:"Object" xml:"Object"`  // THIS IS THE KEY FIELD
	//         RequestId    string                 `json:"RequestId" xml:"RequestId"`
	// }
	// The actual pipelines are likely inside response.Object, e.g., response.Object["Pipelines"]

	if !response.Success {
		return nil, fmt.Errorf("API error: %s (ErrorCode: %s)", response.ErrorMessage, response.ErrorCode)
	}

	if response.Object == nil {
		return []Pipeline{}, nil // No data, but request was successful
	}

	// Try to get the list of pipelines from response.Object. Common key names are "Pipelines", "List", "Items".
	// Let's assume "Pipelines" is the key.
	rawPipelines, ok := response.Object["Pipelines"]
	if !ok {
		return nil, fmt.Errorf("field 'Pipelines' not found in response.Object. Available keys: %v", getMapKeys(response.Object))
	}

	pipelineItems, ok := rawPipelines.([]interface{})
	if !ok {
		return nil, fmt.Errorf("'Pipelines' field in response.Object is not a slice (actual type: %T)", rawPipelines)
	}

	for _, item := range pipelineItems {
		sdkPipeline, ok := item.(map[string]interface{})
		if !ok {
			// Log or skip malformed item
			continue
		}

		var createTime, updateTime time.Time
		// Field names are based on `ListPipelinesPipelines` struct from SDK:
		// Name, PipelineId, Status, CreateTime, UpdateTime, Creator, Modifier.
		// CreateTime, UpdateTime are int64 milliseconds.

		if ct, ok := sdkPipeline["CreateTime"].(float64); ok && ct > 0 { // JSON numbers often decode to float64
			createTime = time.Unix(int64(ct)/1000, 0)
		}
		if ut, ok := sdkPipeline["UpdateTime"].(float64); ok && ut > 0 {
			updateTime = time.Unix(int64(ut)/1000, 0)
		}

		pipelineIdFloat, _ := sdkPipeline["PipelineId"].(float64)

		pipe := Pipeline{
			PipelineID: fmt.Sprintf("%d", int64(pipelineIdFloat)),
			Name:       getStringField(sdkPipeline, "Name"),
			Status:     getStringField(sdkPipeline, "Status"),
			Creator:    getStringField(sdkPipeline, "Creator"),
			Modifier:   getStringField(sdkPipeline, "Modifier"),
			CreateTime: createTime,
			UpdateTime: updateTime,
			// LastRunStatus is not in this response, will need another call or is part of GetPipelineDetails
		}
		pipelines = append(pipelines, pipe)
	}

	return pipelines, nil
}

// listPipelineGroupsWithToken retrieves pipeline groups using personal access token authentication
// Based on: https://help.aliyun.com/zh/yunxiao/developer-reference/listpipelinegroups
func (c *Client) listPipelineGroupsWithToken(organizationId string) ([]PipelineGroup, error) {
	var allGroups []PipelineGroup
	page := 1
	perPage := 30 // Maximum per page according to API docs

	for {
		// API endpoint: GET https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelineGroups
		path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelineGroups?page=%d&perPage=%d", organizationId, page, perPage)

		// Make the request
		url := fmt.Sprintf("https://%s%s", c.endpoint, path)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("x-yunxiao-token", c.personalAccessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if os.Getenv("FLOWT_DEBUG") == "1" {
			debugLogger.Printf("ListPipelineGroups URL: %s", url)
			debugLogger.Printf("Response Status: %d", resp.StatusCode)
			debugLogger.Printf("Response Headers: %v", resp.Header)
			debugLogger.Printf("Response Body: %.1000s", string(respBody))
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		// According to API docs, response is a direct array
		var groupItems []map[string]interface{}
		if err := json.Unmarshal(respBody, &groupItems); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response as array: %w. Response body: %.500s", err, string(respBody))
		}

		// If no items found, we've reached the end
		if len(groupItems) == 0 {
			break
		}

		// Parse each group item according to API documentation
		for _, groupMap := range groupItems {
			// Extract group ID - it might be string or number
			var groupID string
			if id, ok := groupMap["id"].(string); ok {
				groupID = id
			} else if id, ok := groupMap["id"].(float64); ok {
				groupID = fmt.Sprintf("%.0f", id)
			}

			group := PipelineGroup{
				GroupID: groupID,
				Name:    getStringField(groupMap, "name"),
			}

			if group.GroupID != "" && group.Name != "" {
				allGroups = append(allGroups, group)
			}
		}

		// Check pagination headers to determine if there are more pages
		totalPagesHeader := resp.Header.Get("x-total-pages")
		currentPageHeader := resp.Header.Get("x-page")

		if os.Getenv("FLOWT_DEBUG") == "1" {
			debugLogger.Printf("Pagination info - Current page: %s, Total pages: %s, Items in this page: %d", currentPageHeader, totalPagesHeader, len(groupItems))
		}

		// Check if there are more pages using response headers
		if totalPagesHeader != "" && currentPageHeader != "" {
			// Use header information for pagination
			if currentPageHeader == totalPagesHeader {
				// We've reached the last page
				break
			}
		} else {
			// Fallback: if we got fewer items than perPage, we've reached the end
			if len(groupItems) < perPage {
				break
			}
		}

		page++
	}

	return allGroups, nil
}

// ListPipelineGroupPipelines retrieves pipelines within a specific pipeline group
// Based on: https://help.aliyun.com/zh/yunxiao/developer-reference/listpipelinegrouppipelines
func (c *Client) ListPipelineGroupPipelines(organizationId string, groupId int, options map[string]interface{}) ([]Pipeline, error) {
	if !c.useToken {
		return nil, fmt.Errorf("ListPipelineGroupPipelines only supports token-based authentication")
	}

	if organizationId == "" {
		return nil, fmt.Errorf("organizationId is required")
	}

	var allPipelines []Pipeline
	page := 1
	perPage := 30 // Maximum per page according to API docs

	for {
		// API endpoint: GET https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelineGroups/pipelines
		path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelineGroups/pipelines?groupId=%d&page=%d&perPage=%d", organizationId, groupId, page, perPage)

		// Add optional parameters
		if options != nil {
			if createStartTime, ok := options["createStartTime"].(int64); ok {
				path += fmt.Sprintf("&createStartTime=%d", createStartTime)
			}
			if createEndTime, ok := options["createEndTime"].(int64); ok {
				path += fmt.Sprintf("&createEndTime=%d", createEndTime)
			}
			if executeStartTime, ok := options["executeStartTime"].(int64); ok {
				path += fmt.Sprintf("&executeStartTime=%d", executeStartTime)
			}
			if executeEndTime, ok := options["executeEndTime"].(int64); ok {
				path += fmt.Sprintf("&executeEndTime=%d", executeEndTime)
			}
			if pipelineName, ok := options["pipelineName"].(string); ok && pipelineName != "" {
				path += fmt.Sprintf("&pipelineName=%s", pipelineName)
			}
			if statusList, ok := options["statusList"].(string); ok && statusList != "" {
				path += fmt.Sprintf("&statusList=%s", statusList)
			}
		}

		// Make the request
		url := fmt.Sprintf("https://%s%s", c.endpoint, path)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("x-yunxiao-token", c.personalAccessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if os.Getenv("FLOWT_DEBUG") == "1" {
			debugLogger.Printf("ListPipelineGroupPipelines URL: %s", url)
			debugLogger.Printf("Response Status: %d", resp.StatusCode)
			debugLogger.Printf("Response Headers: %v", resp.Header)
			debugLogger.Printf("Response Body: %.1000s", string(respBody))
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		// According to API docs, response is a direct array
		var pipelineItems []map[string]interface{}
		if err := json.Unmarshal(respBody, &pipelineItems); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response as array: %w. Response body: %.500s", err, string(respBody))
		}

		// If no items found, we've reached the end
		if len(pipelineItems) == 0 {
			break
		}

		// Parse each pipeline item according to API documentation
		for _, pipelineMap := range pipelineItems {
			var createTime time.Time
			if ct, ok := pipelineMap["gmtCreate"].(float64); ok && ct > 0 {
				createTime = time.Unix(int64(ct)/1000, 0)
			}

			// Extract pipeline ID - it might be string or number
			var pipelineID string
			if id, ok := pipelineMap["pipelineId"].(string); ok {
				pipelineID = id
			} else if id, ok := pipelineMap["pipelineId"].(float64); ok {
				pipelineID = fmt.Sprintf("%.0f", id)
			}

			pipeline := Pipeline{
				PipelineID: pipelineID,
				Name:       getStringField(pipelineMap, "pipelineName"),
				CreateTime: createTime,
				// Note: This API doesn't return status, creator, etc. - only basic info
			}

			if pipeline.PipelineID != "" && pipeline.Name != "" {
				allPipelines = append(allPipelines, pipeline)
			}
		}

		// Check pagination headers to determine if there are more pages
		totalPagesHeader := resp.Header.Get("x-total-pages")
		currentPageHeader := resp.Header.Get("x-page")

		if os.Getenv("FLOWT_DEBUG") == "1" {
			debugLogger.Printf("Pagination info - Current page: %s, Total pages: %s, Items in this page: %d", currentPageHeader, totalPagesHeader, len(pipelineItems))
		}

		// Check if there are more pages using response headers
		if totalPagesHeader != "" && currentPageHeader != "" {
			// Use header information for pagination
			if currentPageHeader == totalPagesHeader {
				// We've reached the last page
				break
			}
		} else {
			// Fallback: if we got fewer items than perPage, we've reached the end
			if len(pipelineItems) < perPage {
				break
			}
		}

		page++
	}

	return allPipelines, nil
}

// runPipelineWithToken triggers a pipeline run using personal access token authentication
// Based on official API: https://help.aliyun.com/zh/yunxiao/developer-reference/createpipelinerun
func (c *Client) runPipelineWithToken(organizationId, pipelineIdStr string, params map[string]string) (*PipelineRun, error) {
	// Correct API endpoint according to official documentation
	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s/runs", organizationId, pipelineIdStr)

	// Prepare request body according to official API documentation
	// The params should be a JSON string containing pipeline parameters
	var paramsJSON string
	if params != nil && len(params) > 0 {
		// The params map already contains JSON strings for each parameter
		// We need to construct the final JSON object directly
		if runningBranchsJSON, ok := params["runningBranchs"]; ok {
			// runningBranchsJSON is already a JSON string, use it directly
			paramsJSON = fmt.Sprintf("{\"runningBranchs\": %s}", runningBranchsJSON)
		} else {
			// Fallback: marshal the entire params map
			paramsBytes, err := json.Marshal(params)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal params to JSON: %w", err)
			}
			paramsJSON = string(paramsBytes)
		}
	} else {
		// Default empty params
		paramsJSON = "{}"
	}

	requestBody := map[string]interface{}{
		"params": paramsJSON,
	}

	// Make the request directly to handle string response
	url := fmt.Sprintf("https://%s%s", c.endpoint, path)

	var reqBody io.Reader
	if requestBody != nil {
		jsonBody, err := json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest("POST", url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-yunxiao-token", c.personalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

	resp, err := c.httpClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", url, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("RunPipeline Request URL: %s", url)
		debugLogger.Printf("RunPipeline Request Method: POST")
		debugLogger.Printf("RunPipeline Request Headers: %v", req.Header)
		debugLogger.Printf("RunPipeline Response Status: %d", resp.StatusCode)
		debugLogger.Printf("RunPipeline Response Headers: %v", resp.Header)
		debugLogger.Printf("RunPipeline Response Body: %s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// According to user feedback, the response is a bare number (e.g., "21") representing the run ID
	// First, try to parse as string directly
	runID := strings.TrimSpace(string(respBody))

	// Remove quotes if the response is a quoted string
	if len(runID) >= 2 && runID[0] == '"' && runID[len(runID)-1] == '"' {
		runID = runID[1 : len(runID)-1]
	}

	// Validate that the runID is a valid number (since API returns bare numbers)
	if runID != "" {
		// Try to parse as integer to validate it's a number
		if _, err := strconv.ParseInt(runID, 10, 64); err == nil {
			// It's a valid number, use it directly
			if os.Getenv("FLOWT_DEBUG") == "1" {
				debugLogger.Printf("Successfully parsed run ID as number: %s", runID)
			}
		} else {
			// Not a valid number, might be JSON or other format
			if os.Getenv("FLOWT_DEBUG") == "1" {
				debugLogger.Printf("Run ID is not a number, trying JSON parsing: %s", runID)
			}

			// Only try JSON parsing if the response looks like JSON
			if len(respBody) > 0 && (respBody[0] == '{' || respBody[0] == '[') {
				var response map[string]interface{}
				if err := json.Unmarshal(respBody, &response); err == nil {
					// Try to extract run ID from JSON response
					if data, ok := response["data"]; ok {
						if runIdFloat, ok := data.(float64); ok {
							runID = fmt.Sprintf("%.0f", runIdFloat)
						} else if runIdInt, ok := data.(int); ok {
							runID = fmt.Sprintf("%d", runIdInt)
						} else if runIdStr, ok := data.(string); ok {
							runID = runIdStr
						}
					} else if runIdValue, ok := response["runId"]; ok {
						if runIdFloat, ok := runIdValue.(float64); ok {
							runID = fmt.Sprintf("%.0f", runIdFloat)
						} else if runIdInt, ok := runIdValue.(int); ok {
							runID = fmt.Sprintf("%d", runIdInt)
						} else if runIdStr, ok := runIdValue.(string); ok {
							runID = runIdStr
						}
					} else if idValue, ok := response["id"]; ok {
						if runIdFloat, ok := idValue.(float64); ok {
							runID = fmt.Sprintf("%.0f", runIdFloat)
						} else if runIdInt, ok := idValue.(int); ok {
							runID = fmt.Sprintf("%d", runIdInt)
						} else if runIdStr, ok := idValue.(string); ok {
							runID = runIdStr
						}
					} else {
						// Check if response contains the run ID directly as a value
						for _, value := range response {
							if runIdFloat, ok := value.(float64); ok {
								runID = fmt.Sprintf("%.0f", runIdFloat)
								break
							} else if runIdInt, ok := value.(int); ok {
								runID = fmt.Sprintf("%d", runIdInt)
								break
							} else if runIdStr, ok := value.(string); ok {
								runID = runIdStr
								break
							}
						}
					}
				} else {
					if os.Getenv("FLOWT_DEBUG") == "1" {
						debugLogger.Printf("Failed to parse as JSON: %v", err)
					}
				}
			}
		}
	}

	if runID == "" {
		return nil, fmt.Errorf("failed to extract run ID from response. Response body: %s", string(respBody))
	}

	// Return a minimal PipelineRun object
	return &PipelineRun{
		RunID:      runID,
		PipelineID: pipelineIdStr,
		Status:     "RUNNING", // According to API docs, newly created runs are typically RUNNING
	}, nil
}

// Helper function to safely get string fields from map[string]interface{}
func getStringField(data map[string]interface{}, key string) string {
	if val, ok := data[key].(string); ok {
		return val
	}
	return ""
}

// Helper function to get keys from a map for error reporting
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// GetPipelineDetails retrieves details for a specific pipeline.
func (c *Client) GetPipelineDetails(organizationId string, pipelineId string) (*Pipeline, error) {
	// request := devops_rdc.CreateGetPipelineRequest() // Or similar
	// request.OrgId = organizationId
	// request.PipelineId = pipelineId
	// ...
	return nil, fmt.Errorf("not implemented: GetPipelineDetails")
}

// RunPipeline triggers a pipeline run using the ExecutePipeline SDK method.
func (c *Client) RunPipeline(organizationId string, pipelineIdStr string, params map[string]string) (*PipelineRun, error) {
	if organizationId == "" {
		return nil, fmt.Errorf("organizationId is required")
	}
	if pipelineIdStr == "" {
		return nil, fmt.Errorf("pipelineId is required")
	}

	// Use different methods based on authentication type
	if c.useToken {
		return c.runPipelineWithToken(organizationId, pipelineIdStr, params)
	}

	// Use SDK for AccessKey authentication
	pipelineIdInt, err := strconv.ParseInt(pipelineIdStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pipelineId to int64: %w", err)
	}

	request := devops_rdc.CreateExecutePipelineRequest()
	request.Scheme = "https"
	request.OrgId = organizationId
	request.PipelineId = requests.NewInteger(int(pipelineIdInt))

	// Convert params map to "key1=value1,key2=value2" string format
	// TODO: Confirm the exact format required by the Aliyun API for Parameters.
	// Assuming "key1=value1,key2=value2" or JSON string. For now, using the former.
	var paramList []string
	for key, value := range params {
		paramList = append(paramList, fmt.Sprintf("%s=%s", key, value))
	}
	request.Parameters = strings.Join(paramList, ",") // Example format

	response, err := c.sdkClient.ExecutePipeline(request)
	if err != nil {
		return nil, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("API error executing pipeline: %s (ErrorCode: %s)", response.ErrorMessage, response.ErrorCode)
	}

	// The response.Object is the RunId (int64)
	runId := response.Object

	// The ExecutePipeline API only returns the RunID.
	// To return a full PipelineRun struct, we should ideally call GetPipelineRun here.
	// However, to keep this method focused, we'll return a minimal PipelineRun object.
	// The caller can then use GetPipelineRun to fetch full details if needed.
	return &PipelineRun{
		RunID:      fmt.Sprintf("%d", runId),
		PipelineID: pipelineIdStr,
		Status:     "QUEUED", // Assuming it's queued; actual status needs GetPipelineRun
		// StartTime would be set by GetPipelineRun
	}, nil
}

// StopPipelineRun stops a pipeline run.
func (c *Client) StopPipelineRun(organizationId string, pipelineId string, runId string) error {
	if organizationId == "" {
		return fmt.Errorf("organizationId is required")
	}
	if pipelineId == "" {
		return fmt.Errorf("pipelineId is required")
	}
	if runId == "" {
		return fmt.Errorf("runId is required")
	}

	// Use different methods based on authentication type
	if c.useToken {
		return c.stopPipelineRunWithToken(organizationId, pipelineId, runId)
	}

	// TODO: Implement SDK-based method for AccessKey authentication
	return fmt.Errorf("StopPipelineRun with AccessKey authentication not implemented yet")
}

// stopPipelineRunWithToken stops a pipeline run using personal access token authentication
// Based on official API: https://help.aliyun.com/zh/yunxiao/developer-reference/updatepipelinerun
func (c *Client) stopPipelineRunWithToken(organizationId, pipelineId, runId string) error {
	// API endpoint: PUT https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelines/{pipelineId}/runs/{pipelineRunId}
	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s/runs/%s", organizationId, pipelineId, runId)
	url := fmt.Sprintf("https://%s%s", c.endpoint, path)

	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-yunxiao-token", c.personalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("StopPipelineRun URL: %s", url)
		debugLogger.Printf("Response Status: %d", resp.StatusCode)
		debugLogger.Printf("Response Body: %s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// According to API documentation, response is a boolean indicating success
	var success bool
	if err := json.Unmarshal(respBody, &success); err != nil {
		// If response is not a boolean, try to parse as string "true"/"false"
		responseStr := strings.TrimSpace(string(respBody))
		responseStr = strings.Trim(responseStr, "\"") // Remove quotes if present
		if responseStr == "true" {
			success = true
		} else if responseStr == "false" {
			success = false
		} else {
			return fmt.Errorf("failed to parse response as boolean: %w. Response: %s", err, string(respBody))
		}
	}

	if !success {
		return fmt.Errorf("failed to stop pipeline run: API returned false")
	}

	return nil
}

// PipelineRunInfo contains detailed information about a pipeline run including repository information
type PipelineRunInfo struct {
	*PipelineRun
	RepositoryURLs map[string]string // Map of repository URL to branch name from last run
}

// GetLatestPipelineRun retrieves the latest pipeline run information
// Based on official API: https://help.aliyun.com/zh/yunxiao/developer-reference/getlatestpipelinerun
func (c *Client) GetLatestPipelineRun(organizationId, pipelineId string) (*PipelineRun, error) {
	if organizationId == "" {
		return nil, fmt.Errorf("organizationId is required")
	}
	if pipelineId == "" {
		return nil, fmt.Errorf("pipelineId is required")
	}

	// Use token-based authentication (only supported method for this API)
	if !c.useToken {
		return nil, fmt.Errorf("GetLatestPipelineRun only supports personal access token authentication")
	}

	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s/runs/latestPipelineRun", organizationId, pipelineId)

	response, err := c.makeTokenRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest pipeline run: %w", err)
	}

	// Parse the response according to the API documentation
	run := &PipelineRun{}

	// Extract basic run information
	if pipelineRunId, ok := response["pipelineRunId"]; ok {
		if runIdFloat, ok := pipelineRunId.(float64); ok {
			run.RunID = fmt.Sprintf("%.0f", runIdFloat)
		} else if runIdStr, ok := pipelineRunId.(string); ok {
			run.RunID = runIdStr
		}
	}

	if pipelineIdValue, ok := response["pipelineId"]; ok {
		if pipelineIdFloat, ok := pipelineIdValue.(float64); ok {
			run.PipelineID = fmt.Sprintf("%.0f", pipelineIdFloat)
		} else if pipelineIdStr, ok := pipelineIdValue.(string); ok {
			run.PipelineID = pipelineIdStr
		}
	}

	if status, ok := response["status"].(string); ok {
		run.Status = status
	}

	// Parse trigger mode
	if triggerMode, ok := response["triggerMode"]; ok {
		if triggerModeFloat, ok := triggerMode.(float64); ok {
			switch int(triggerModeFloat) {
			case 1:
				run.TriggerMode = "MANUAL"
			case 2:
				run.TriggerMode = "SCHEDULE"
			case 3:
				run.TriggerMode = "PUSH"
			case 5:
				run.TriggerMode = "PIPELINE"
			case 6:
				run.TriggerMode = "WEBHOOK"
			default:
				run.TriggerMode = fmt.Sprintf("UNKNOWN_%d", int(triggerModeFloat))
			}
		}
	}

	// Parse timestamps
	if createTime, ok := response["createTime"]; ok {
		if createTimeFloat, ok := createTime.(float64); ok {
			run.StartTime = time.Unix(int64(createTimeFloat)/1000, 0)
		}
	}

	if endTime, ok := response["endTime"]; ok {
		if endTimeFloat, ok := endTime.(float64); ok {
			run.FinishTime = time.Unix(int64(endTimeFloat)/1000, 0)
		}
	}

	return run, nil
}

// GetLatestPipelineRunInfo retrieves the latest pipeline run information with repository details
func (c *Client) GetLatestPipelineRunInfo(organizationId, pipelineId string) (*PipelineRunInfo, error) {
	if organizationId == "" {
		return nil, fmt.Errorf("organizationId is required")
	}
	if pipelineId == "" {
		return nil, fmt.Errorf("pipelineId is required")
	}

	// Use token-based authentication (only supported method for this API)
	if !c.useToken {
		return nil, fmt.Errorf("GetLatestPipelineRunInfo only supports personal access token authentication")
	}

	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s/runs/latestPipelineRun", organizationId, pipelineId)

	response, err := c.makeTokenRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest pipeline run: %w", err)
	}

	// Parse the response according to the API documentation
	run := &PipelineRun{}
	runInfo := &PipelineRunInfo{
		PipelineRun:    run,
		RepositoryURLs: make(map[string]string),
	}

	// Extract basic run information
	if pipelineRunId, ok := response["pipelineRunId"]; ok {
		if runIdFloat, ok := pipelineRunId.(float64); ok {
			run.RunID = fmt.Sprintf("%.0f", runIdFloat)
		} else if runIdStr, ok := pipelineRunId.(string); ok {
			run.RunID = runIdStr
		}
	}

	if pipelineIdValue, ok := response["pipelineId"]; ok {
		if pipelineIdFloat, ok := pipelineIdValue.(float64); ok {
			run.PipelineID = fmt.Sprintf("%.0f", pipelineIdFloat)
		} else if pipelineIdStr, ok := pipelineIdValue.(string); ok {
			run.PipelineID = pipelineIdStr
		}
	}

	if status, ok := response["status"].(string); ok {
		run.Status = status
	}

	// Parse trigger mode
	if triggerMode, ok := response["triggerMode"]; ok {
		if triggerModeFloat, ok := triggerMode.(float64); ok {
			switch int(triggerModeFloat) {
			case 1:
				run.TriggerMode = "MANUAL"
			case 2:
				run.TriggerMode = "SCHEDULE"
			case 3:
				run.TriggerMode = "PUSH"
			case 5:
				run.TriggerMode = "PIPELINE"
			case 6:
				run.TriggerMode = "WEBHOOK"
			default:
				run.TriggerMode = fmt.Sprintf("UNKNOWN_%d", int(triggerModeFloat))
			}
		}
	}

	// Parse timestamps
	if createTime, ok := response["createTime"]; ok {
		if createTimeFloat, ok := createTime.(float64); ok {
			run.StartTime = time.Unix(int64(createTimeFloat)/1000, 0)
		}
	}

	if endTime, ok := response["endTime"]; ok {
		if endTimeFloat, ok := endTime.(float64); ok {
			run.FinishTime = time.Unix(int64(endTimeFloat)/1000, 0)
		}
	}

	// Extract repository information from sources
	if sources, ok := response["sources"]; ok {
		if sourcesArray, ok := sources.([]interface{}); ok {
			for _, source := range sourcesArray {
				if sourceMap, ok := source.(map[string]interface{}); ok {
					// Check for repository URL in data.repo field (new structure)
					if dataMap, ok := sourceMap["data"].(map[string]interface{}); ok {
						if repoUrl, ok := dataMap["repo"].(string); ok {
							// Extract branch information from data.branch
							branch := "master" // default branch
							if branchInfo, ok := dataMap["branch"].(string); ok && branchInfo != "" {
								branch = branchInfo
							}
							runInfo.RepositoryURLs[repoUrl] = branch
							if os.Getenv("FLOWT_DEBUG") == "1" {
								debugLogger.Printf("Extracted repository from sources[].data: %s -> %s", repoUrl, branch)
							}
						}
					} else if repoUrl, ok := sourceMap["repoUrl"].(string); ok {
						// Fallback: check for direct repoUrl field (old structure)
						branch := "master" // default branch
						if branchInfo, ok := sourceMap["branch"].(string); ok && branchInfo != "" {
							branch = branchInfo
						} else if branchInfo, ok := sourceMap["branchName"].(string); ok && branchInfo != "" {
							branch = branchInfo
						}
						runInfo.RepositoryURLs[repoUrl] = branch
						if os.Getenv("FLOWT_DEBUG") == "1" {
							debugLogger.Printf("Extracted repository from sources[].repoUrl: %s -> %s", repoUrl, branch)
						}
					}
				}
			}
		}
	}

	// If no repository information found in sources, try to extract from other fields
	if len(runInfo.RepositoryURLs) == 0 {
		// Try to extract from pipeline configuration or other fields
		// This might be in different locations depending on the API response structure
		if pipelineConfig, ok := response["pipelineConfig"]; ok {
			if configMap, ok := pipelineConfig.(map[string]interface{}); ok {
				if sources, ok := configMap["sources"]; ok {
					if sourcesArray, ok := sources.([]interface{}); ok {
						for _, source := range sourcesArray {
							if sourceMap, ok := source.(map[string]interface{}); ok {
								// Check for repository URL in data.repo field (new structure)
								if dataMap, ok := sourceMap["data"].(map[string]interface{}); ok {
									if repoUrl, ok := dataMap["repo"].(string); ok {
										branch := "master"
										if branchInfo, ok := dataMap["branch"].(string); ok && branchInfo != "" {
											branch = branchInfo
										}
										runInfo.RepositoryURLs[repoUrl] = branch
									}
								} else if repoUrl, ok := sourceMap["repoUrl"].(string); ok {
									// Fallback: check for direct repoUrl field (old structure)
									branch := "master"
									if branchInfo, ok := sourceMap["branch"].(string); ok && branchInfo != "" {
										branch = branchInfo
									}
									runInfo.RepositoryURLs[repoUrl] = branch
								}
							}
						}
					}
				}
			}
		}
	}

	return runInfo, nil
}

// GetPipelineRun retrieves details of a specific pipeline run using GetPipelineInstanceInfo SDK method.
func (c *Client) GetPipelineRun(organizationId string, pipelineIdStr string, runIdStr string) (*PipelineRun, error) {
	if organizationId == "" {
		return nil, fmt.Errorf("organizationId is required")
	}
	if pipelineIdStr == "" {
		return nil, fmt.Errorf("pipelineId is required")
	}
	if runIdStr == "" {
		return nil, fmt.Errorf("runId is required")
	}

	// Use different methods based on authentication type
	if c.useToken {
		return c.getPipelineRunWithToken(organizationId, pipelineIdStr, runIdStr)
	}

	// Use SDK for AccessKey authentication
	pipelineIdInt, err := strconv.ParseInt(pipelineIdStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pipelineId to int64: %w", err)
	}

	request := devops_rdc.CreateGetPipelineInstanceInfoRequest()
	request.Scheme = "https"
	request.OrgId = organizationId
	request.PipelineId = requests.NewInteger(int(pipelineIdInt))
	request.FlowInstanceId = runIdStr // FlowInstanceId is the RunId

	response, err := c.sdkClient.GetPipelineInstanceInfo(request)
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline instance info: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("API error getting pipeline instance info: %s (ErrorCode: %s)", response.ErrorMessage, response.ErrorCode)
	}

	// response.Object is of type devops_rdc.Object (which is an alias for struct{} in the generated code if not defined)
	// Need to parse this dynamically, similar to ListPipelineGroups
	dataBytes, err := json.Marshal(response.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response.Object for GetPipelineRun: %w", err)
	}

	var runMap map[string]interface{}
	if err := json.Unmarshal(dataBytes, &runMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response.Object into map for GetPipelineRun: %w. Raw JSON: %s", err, string(dataBytes))
	}

	// Extract fields from runMap
	// Common fields from a pipeline run instance:
	// Status, StartTime, FinishTime, TriggerMode, RunId (usually FlowInstanceId itself), PipelineId
	// Timestamps might be in milliseconds (float64) or string format.
	// Status might be "SUCCESS", "RUNNING", "FAILED", "CANCELED", "QUEUED" etc.

	var startTime, finishTime time.Time
	if st, ok := runMap["StartTime"].(float64); ok { // Assuming milliseconds
		startTime = time.Unix(int64(st)/1000, 0)
	} else if stStr, ok := runMap["StartTime"].(string); ok {
		startTime, _ = time.Parse(time.RFC3339Nano, stStr) // Or other common time formats
	}
	// Similar for FinishTime
	if ft, ok := runMap["FinishTime"].(float64); ok {
		finishTime = time.Unix(int64(ft)/1000, 0)
	} else if ftStr, ok := runMap["FinishTime"].(string); ok {
		finishTime, _ = time.Parse(time.RFC3339Nano, ftStr)
	}

	// Assuming `Id` or `FlowInstanceId` for RunID from the map.
	// `PipelineId` should also be present.
	// `Status` and `TriggerMode` are important.

	pipelineRun := &PipelineRun{
		RunID:       getStringField(runMap, "Id"),                                   // Or "FlowInstanceId", "InstanceId"
		PipelineID:  fmt.Sprintf("%d", int64(getNumberField(runMap, "PipelineId"))), // Assuming "PipelineId" is a number
		Status:      getStringField(runMap, "Status"),
		TriggerMode: getStringField(runMap, "TriggerMode"), // Or "triggerMode", "triggerType"
		StartTime:   startTime,
		FinishTime:  finishTime,
	}

	// If RunID was not found by "Id", try "FlowInstanceId"
	if pipelineRun.RunID == "" {
		pipelineRun.RunID = getStringField(runMap, "FlowInstanceId")
	}
	if pipelineRun.RunID == "" { // Fallback to the input runId if not found in response
		pipelineRun.RunID = runIdStr
	}
	if pipelineRun.PipelineID == "0" || pipelineRun.PipelineID == "" { // Fallback for PipelineID
		pipelineRun.PipelineID = pipelineIdStr
	}

	// The subtask requires mapping Status correctly.
	// Statuses like "SUCCESS", "FAILED", "RUNNING", "CANCELED", "WAITING", "QUEUED" are common.
	// The `getStringField(runMap, "Status")` should capture this if the key "Status" is correct.

	return pipelineRun, nil
}

// GetPipelineRunDetails retrieves detailed information about a pipeline run including job list
// Based on: https://help.aliyun.com/zh/yunxiao/developer-reference/getpipelinerun
func (c *Client) GetPipelineRunDetails(organizationId, pipelineId, pipelineRunId string) (*PipelineRunDetails, error) {
	if !c.useToken {
		return nil, fmt.Errorf("GetPipelineRunDetails only supports token-based authentication")
	}

	if organizationId == "" || pipelineId == "" || pipelineRunId == "" {
		return nil, fmt.Errorf("organizationId, pipelineId, and pipelineRunId are required")
	}

	// API endpoint: GET https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelines/{pipelineId}/runs/{pipelineRunId}
	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s/runs/%s", organizationId, pipelineId, pipelineRunId)
	url := fmt.Sprintf("https://%s%s", c.endpoint, path)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-yunxiao-token", c.personalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("GetPipelineRunDetails URL: %s", url)
		debugLogger.Printf("Response Status: %d", resp.StatusCode)
		debugLogger.Printf("Response Body: %.1000s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var responseData map[string]interface{}
	if err := json.Unmarshal(respBody, &responseData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Parse the response according to API documentation
	details := &PipelineRunDetails{}

	if pipelineRunID, ok := responseData["pipelineRunId"].(float64); ok {
		details.PipelineRunID = int64(pipelineRunID)
	}
	if pipelineID, ok := responseData["pipelineId"].(float64); ok {
		details.PipelineID = int64(pipelineID)
	}
	if status, ok := responseData["status"].(string); ok {
		details.Status = status
	}
	if triggerMode, ok := responseData["triggerMode"].(float64); ok {
		details.TriggerMode = int(triggerMode)
	}
	if createTime, ok := responseData["createTime"].(float64); ok {
		details.CreateTime = int64(createTime)
	}
	if updateTime, ok := responseData["updateTime"].(float64); ok {
		details.UpdateTime = int64(updateTime)
	}

	// Parse stages and jobs
	if stagesData, ok := responseData["stages"].([]interface{}); ok {
		for _, stageItem := range stagesData {
			if stageMap, ok := stageItem.(map[string]interface{}); ok {
				stage := Stage{}
				if index, ok := stageMap["index"].(string); ok {
					stage.Index = index
				}
				if name, ok := stageMap["name"].(string); ok {
					stage.Name = name
				}

				// Parse stage info and jobs
				if stageInfo, ok := stageMap["stageInfo"].(map[string]interface{}); ok {
					if jobsData, ok := stageInfo["jobs"].([]interface{}); ok {
						for _, jobItem := range jobsData {
							if jobMap, ok := jobItem.(map[string]interface{}); ok {
								job := Job{}
								if id, ok := jobMap["id"].(float64); ok {
									job.ID = int64(id)
								}
								if jobSign, ok := jobMap["jobSign"].(string); ok {
									job.JobSign = jobSign
								}
								if name, ok := jobMap["name"].(string); ok {
									job.Name = name
								}
								if status, ok := jobMap["status"].(string); ok {
									job.Status = status
								}
								if startTime, ok := jobMap["startTime"].(float64); ok && startTime > 0 {
									job.StartTime = time.Unix(int64(startTime)/1000, 0)
								}
								if endTime, ok := jobMap["endTime"].(float64); ok && endTime > 0 {
									job.EndTime = time.Unix(int64(endTime)/1000, 0)
								}
								if result, ok := jobMap["result"].(string); ok {
									job.Result = result
								}

								// Parse actions array
								if actionsData, ok := jobMap["actions"].([]interface{}); ok {
									for _, actionItem := range actionsData {
										if actionMap, ok := actionItem.(map[string]interface{}); ok {
											action := JobAction{}
											if actionType, ok := actionMap["type"].(string); ok {
												action.Type = actionType
											}
											if displayType, ok := actionMap["displayType"].(string); ok {
												action.DisplayType = displayType
											}
											if data, ok := actionMap["data"].(string); ok {
												action.Data = data
											}
											if disable, ok := actionMap["disable"].(bool); ok {
												action.Disable = disable
											}
											if params, ok := actionMap["params"].(map[string]interface{}); ok {
												action.Params = params
											}
											if name, ok := actionMap["name"].(string); ok {
												action.Name = name
											}
											if title, ok := actionMap["title"].(string); ok {
												action.Title = title
											}
											action.Order = actionMap["order"]
											job.Actions = append(job.Actions, action)
										}
									}
								}
								stage.Jobs = append(stage.Jobs, job)
							}
						}
					}
				}
				details.Stages = append(details.Stages, stage)
			}
		}
	}

	return details, nil
}

// GetPipelineJobRunLog retrieves logs for a specific job within a pipeline run
// Based on: https://help.aliyun.com/zh/yunxiao/developer-reference/getpipelinejobrunlog
func (c *Client) GetPipelineJobRunLog(organizationId, pipelineId, pipelineRunId, jobId string) (string, error) {
	if !c.useToken {
		return "", fmt.Errorf("GetPipelineJobRunLog only supports token-based authentication")
	}

	if organizationId == "" || pipelineId == "" || pipelineRunId == "" || jobId == "" {
		return "", fmt.Errorf("organizationId, pipelineId, pipelineRunId, and jobId are required")
	}

	// API endpoint: GET https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelines/{pipelineId}/runs/{pipelineRunId}/job/{jobId}/log
	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s/runs/%s/job/%s/log", organizationId, pipelineId, pipelineRunId, jobId)
	url := fmt.Sprintf("https://%s%s", c.endpoint, path)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-yunxiao-token", c.personalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("GetPipelineJobRunLog URL: %s", url)
		debugLogger.Printf("Response Status: %d", resp.StatusCode)
		debugLogger.Printf("Response Body: %.1000s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var responseData map[string]interface{}
	if err := json.Unmarshal(respBody, &responseData); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Extract log content from response
	if content, ok := responseData["content"].(string); ok {
		return content, nil
	}

	return "", fmt.Errorf("no log content found in response")
}

// GetPipelineRunLogs retrieves logs for all jobs within a pipeline run.
// This method first gets the pipeline run details to obtain the job list,
// then fetches logs for each job and concatenates them with job headers.
func (c *Client) GetPipelineRunLogs(organizationId string, pipelineIdStr string, runIdStr string) (string, error) {
	if !c.useToken {
		return "", fmt.Errorf("GetPipelineRunLogs only supports token-based authentication")
	}

	if organizationId == "" || pipelineIdStr == "" || runIdStr == "" {
		return "", fmt.Errorf("organizationId, pipelineId, and runId are required for GetPipelineRunLogs")
	}

	// Step 1: Get pipeline run details to obtain job list
	runDetails, err := c.GetPipelineRunDetails(organizationId, pipelineIdStr, runIdStr)
	if err != nil {
		return "", fmt.Errorf("failed to get pipeline run details: %w", err)
	}

	var allLogs strings.Builder
	allLogs.WriteString(fmt.Sprintf("Pipeline Run Logs - Run ID: %s\n", runIdStr))
	allLogs.WriteString(fmt.Sprintf("Pipeline ID: %s\n", pipelineIdStr))
	allLogs.WriteString(fmt.Sprintf("Status: %s\n", runDetails.Status))
	allLogs.WriteString("=" + strings.Repeat("=", 80) + "\n\n")

	// Step 2: Iterate through all stages and jobs to fetch logs
	jobCount := 0
	for _, stage := range runDetails.Stages {
		if len(stage.Jobs) > 0 {
			allLogs.WriteString(fmt.Sprintf("[yellow]Stage: %s (%s)[-]\n", stage.Name, stage.Index))
			allLogs.WriteString("-" + strings.Repeat("-", 60) + "\n\n")
		}

		for _, job := range stage.Jobs {
			jobCount++

			// Add job header with yellow color formatting for tview
			allLogs.WriteString(fmt.Sprintf("[yellow]Job #%d: %s (ID: %d)[-]\n", jobCount, job.Name, job.ID))
			allLogs.WriteString(fmt.Sprintf("[yellow]Job Sign: %s[-]\n", job.JobSign))
			allLogs.WriteString(fmt.Sprintf("[yellow]Status: %s[-]\n", job.Status))
			if !job.StartTime.IsZero() {
				allLogs.WriteString(fmt.Sprintf("[yellow]Start Time: %s[-]\n", job.StartTime.Format("2006-01-02 15:04:05")))
			}
			if !job.EndTime.IsZero() {
				allLogs.WriteString(fmt.Sprintf("[yellow]End Time: %s[-]\n", job.EndTime.Format("2006-01-02 15:04:05")))
			}
			allLogs.WriteString("[yellow]" + strings.Repeat("=", 50) + "[-]\n")

			// Step 3: Fetch logs for this specific job
			// Check if this job has GetVMDeployOrder action
			hasVMDeployAction := false
			for _, action := range job.Actions {
				if action.Type == "GetVMDeployOrder" {
					hasVMDeployAction = true
					break
				}
			}

			if hasVMDeployAction {
				// This is a VM deployment job, use VM deployment APIs
				deployOrderId, err := extractDeployOrderIdFromActions(job.Actions)
				if err != nil {
					allLogs.WriteString(fmt.Sprintf("Error extracting deployOrderId from job actions: %v\n", err))
					// For running deployments, the deployOrderId might not be available yet
					if job.Status == "RUNNING" || job.Status == "QUEUED" {
						allLogs.WriteString("Deployment is still in progress. Deploy order information will be available once the deployment starts.\n")
					} else if job.Status == "FAILED" {
						allLogs.WriteString("Deployment job failed. No deploy order information available.\n")
					} else {
						allLogs.WriteString("Deploy order information is not available for this job.\n")
					}
					// Continue processing other jobs instead of stopping
				} else {
					// Get VM deployment order details
					deployOrder, err := c.GetVMDeployOrder(organizationId, pipelineIdStr, deployOrderId)
					if err != nil {
						allLogs.WriteString(fmt.Sprintf("Error fetching VM deploy order %s: %v\n", deployOrderId, err))
						allLogs.WriteString("Unable to retrieve deployment details at this time.\n")
					} else {
						allLogs.WriteString(fmt.Sprintf("[yellow]Deploy Order ID: %d[-]\n", deployOrder.DeployOrderId))
						allLogs.WriteString(fmt.Sprintf("[yellow]Deploy Status: %s[-]\n", deployOrder.Status))
						allLogs.WriteString(fmt.Sprintf("[yellow]Current Batch: %d/%d[-]\n", deployOrder.CurrentBatch, deployOrder.TotalBatch))
						allLogs.WriteString(fmt.Sprintf("[yellow]Host Group ID: %d[-]\n", deployOrder.DeployMachineInfo.HostGroupId))
						allLogs.WriteString("[yellow]" + strings.Repeat("-", 40) + "[-]\n")

						// Get logs for each machine in the deployment
						if len(deployOrder.DeployMachineInfo.DeployMachines) == 0 {
							allLogs.WriteString("No machines found in this deployment.\n")
						} else {
							for i, machine := range deployOrder.DeployMachineInfo.DeployMachines {
								allLogs.WriteString(fmt.Sprintf("[yellow]Machine #%d: %s (SN: %s)[-]\n", i+1, machine.IP, machine.MachineSn))
								allLogs.WriteString(fmt.Sprintf("[yellow]Machine Status: %s, Client Status: %s[-]\n", machine.Status, machine.ClientStatus))
								allLogs.WriteString(fmt.Sprintf("[yellow]Batch: %d[-]\n", machine.BatchNum))
								allLogs.WriteString("[yellow]" + strings.Repeat(".", 30) + "[-]\n")

								// Get machine deployment log
								machineLog, err := c.GetVMDeployMachineLog(organizationId, pipelineIdStr, deployOrderId, machine.MachineSn)
								if err != nil {
									allLogs.WriteString(fmt.Sprintf("Error fetching machine log for %s: %v\n", machine.MachineSn, err))
								} else {
									if machineLog.DeployBeginTime != "" {
										allLogs.WriteString(fmt.Sprintf("Deploy Begin Time: %s\n", machineLog.DeployBeginTime))
									}
									if machineLog.DeployEndTime != "" {
										allLogs.WriteString(fmt.Sprintf("Deploy End Time: %s\n", machineLog.DeployEndTime))
									}
									if machineLog.AliyunRegion != "" {
										allLogs.WriteString(fmt.Sprintf("Region: %s\n", machineLog.AliyunRegion))
									}
									if machineLog.DeployLogPath != "" {
										allLogs.WriteString(fmt.Sprintf("Log Path: %s\n", machineLog.DeployLogPath))
									}
									allLogs.WriteString("Deploy Log:\n")
									if machineLog.DeployLog == "" {
										allLogs.WriteString("No deployment logs available for this machine.\n")
									} else {
										allLogs.WriteString(machineLog.DeployLog)
										if !strings.HasSuffix(machineLog.DeployLog, "\n") {
											allLogs.WriteString("\n")
										}
									}
								}
								allLogs.WriteString("\n")
							}
						}
					}
				}
			} else {
				// This is a regular job, use standard job log API
				jobIdStr := fmt.Sprintf("%d", job.ID)
				jobLogs, err := c.GetPipelineJobRunLog(organizationId, pipelineIdStr, runIdStr, jobIdStr)
				if err != nil {
					allLogs.WriteString(fmt.Sprintf("Error fetching logs for job %s: %v\n", jobIdStr, err))
				} else if jobLogs == "" {
					allLogs.WriteString("No logs available for this job.\n")
				} else {
					allLogs.WriteString(jobLogs)
					if !strings.HasSuffix(jobLogs, "\n") {
						allLogs.WriteString("\n")
					}
				}
			}

			allLogs.WriteString("\n" + strings.Repeat("=", 80) + "\n\n")
		}
	}

	if jobCount == 0 {
		allLogs.WriteString("No jobs found in this pipeline run.\n")
	} else {
		allLogs.WriteString(fmt.Sprintf("Total jobs processed: %d\n", jobCount))
	}

	return allLogs.String(), nil
}

// getNumberField is a helper for dynamic map parsing
func getNumberField(data map[string]interface{}, key string) float64 {
	if val, ok := data[key].(float64); ok { // JSON numbers are often float64
		return val
	}
	// Could add more type checks if needed (e.g., string to float64)
	return 0
}

// extractDeployOrderId extracts deployOrderId from job result JSON string
func extractDeployOrderId(resultJSON string) (string, error) {
	if resultJSON == "" {
		return "", fmt.Errorf("result JSON is empty")
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal result JSON: %w", err)
	}

	// Add debug logging if enabled
	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("Parsing job result JSON for deployOrderId: %.200s", resultJSON)
	}

	// Navigate through the nested structure: result.data.deployOrderId.id
	if data, ok := result["data"].(map[string]interface{}); ok {
		if deployOrderIdData, ok := data["deployOrderId"].(map[string]interface{}); ok {
			if id, ok := deployOrderIdData["id"].(float64); ok {
				return fmt.Sprintf("%.0f", id), nil
			}
			if id, ok := deployOrderIdData["id"].(string); ok {
				return id, nil
			}
		} else {
			// Check if deployOrderId is directly under data as a number or string
			if id, ok := data["deployOrderId"].(float64); ok {
				return fmt.Sprintf("%.0f", id), nil
			}
			if id, ok := data["deployOrderId"].(string); ok {
				return id, nil
			}
		}
	}

	// Try alternative structure: result.deployOrderId
	if id, ok := result["deployOrderId"].(float64); ok {
		return fmt.Sprintf("%.0f", id), nil
	}
	if id, ok := result["deployOrderId"].(string); ok {
		return id, nil
	}

	// Try alternative structure: result.deployOrderId.id
	if deployOrderIdData, ok := result["deployOrderId"].(map[string]interface{}); ok {
		if id, ok := deployOrderIdData["id"].(float64); ok {
			return fmt.Sprintf("%.0f", id), nil
		}
		if id, ok := deployOrderIdData["id"].(string); ok {
			return id, nil
		}
	}

	return "", fmt.Errorf("deployOrderId not found in result JSON. Available keys: %v", getMapKeys(result))
}

// extractDeployOrderIdFromActions extracts deployOrderId from job actions array
// Based on the API response structure where deployOrderId is in actions[].data or actions[].params
func extractDeployOrderIdFromActions(actions []JobAction) (string, error) {
	if len(actions) == 0 {
		return "", fmt.Errorf("no actions found in job")
	}

	// Add debug logging if enabled
	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("Parsing job actions for deployOrderId, found %d actions", len(actions))
	}

	// Look for GetVMDeployOrder action
	for _, action := range actions {
		if action.Type == "GetVMDeployOrder" {
			// First try to get deployOrderId from action.params
			if action.Params != nil {
				if deployOrderId, ok := action.Params["deployOrderId"]; ok {
					if id, ok := deployOrderId.(float64); ok {
						return fmt.Sprintf("%.0f", id), nil
					}
					if id, ok := deployOrderId.(string); ok {
						return id, nil
					}
				}
			}

			// Then try to parse from action.data JSON string
			if action.Data != "" {
				var actionData map[string]interface{}
				if err := json.Unmarshal([]byte(action.Data), &actionData); err != nil {
					if os.Getenv("FLOWT_DEBUG") == "1" {
						debugLogger.Printf("Failed to unmarshal action data: %v, data: %.200s", err, action.Data)
					}
					continue
				}

				// Look for deployOrderId in the parsed data
				if deployOrderId, ok := actionData["deployOrderId"]; ok {
					if id, ok := deployOrderId.(float64); ok {
						return fmt.Sprintf("%.0f", id), nil
					}
					if id, ok := deployOrderId.(string); ok {
						return id, nil
					}
				}

				if os.Getenv("FLOWT_DEBUG") == "1" {
					debugLogger.Printf("Action data keys: %v", getMapKeys(actionData))
				}
			}

			if os.Getenv("FLOWT_DEBUG") == "1" {
				debugLogger.Printf("Found GetVMDeployOrder action but no deployOrderId found in params or data")
			}
		}
	}

	return "", fmt.Errorf("deployOrderId not found in any GetVMDeployOrder action")
}

// PipelineRunsResult represents a paginated result of pipeline runs
type PipelineRunsResult struct {
	Runs        []PipelineRun
	CurrentPage int
	TotalPages  int
	PerPage     int
	TotalRuns   int
	HasMore     bool
}

// ListPipelineRunsPaginated retrieves a single page of runs for a specific pipeline.
func (c *Client) ListPipelineRunsPaginated(organizationId, pipelineId string, page, perPage int) (*PipelineRunsResult, error) {
	if organizationId == "" {
		return nil, fmt.Errorf("organizationId is required for ListPipelineRunsPaginated")
	}
	if pipelineId == "" {
		return nil, fmt.Errorf("pipelineId is required for ListPipelineRunsPaginated")
	}

	if !c.useToken {
		return nil, fmt.Errorf("ListPipelineRunsPaginated with AccessKey authentication not implemented yet")
	}

	return c.listPipelineRunsPaginatedWithToken(organizationId, pipelineId, page, perPage)
}

// listPipelineRunsPaginatedWithToken retrieves a single page of pipeline runs
func (c *Client) listPipelineRunsPaginatedWithToken(organizationId, pipelineId string, page, perPage int) (*PipelineRunsResult, error) {
	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s/runs?page=%d&perPage=%d", organizationId, pipelineId, page, perPage)

	url := fmt.Sprintf("https://%s%s", c.endpoint, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-yunxiao-token", c.personalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("Trying pipeline runs endpoint: %s", url)
		debugLogger.Printf("Response Status: %d", resp.StatusCode)
		debugLogger.Printf("Response Body: %.500s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse the response array
	var runItems []map[string]interface{}
	if err := json.Unmarshal(respBody, &runItems); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response as array: %w. Response body: %.500s", err, string(respBody))
	}

	var pipelineRuns []PipelineRun
	for _, runMap := range runItems {
		var startTime, finishTime time.Time

		if st, ok := runMap["startTime"].(float64); ok && st > 0 {
			startTime = time.Unix(int64(st)/1000, 0)
		}
		if ft, ok := runMap["endTime"].(float64); ok && ft > 0 {
			finishTime = time.Unix(int64(ft)/1000, 0)
		}

		var runID string
		if id, ok := runMap["pipelineRunId"].(string); ok {
			runID = id
		} else if id, ok := runMap["pipelineRunId"].(float64); ok {
			runID = fmt.Sprintf("%.0f", id)
		}

		var pipelineID string
		if pid, ok := runMap["pipelineId"].(string); ok {
			pipelineID = pid
		} else if pid, ok := runMap["pipelineId"].(float64); ok {
			pipelineID = fmt.Sprintf("%.0f", pid)
		}

		var triggerMode string
		if tm, ok := runMap["triggerMode"].(float64); ok {
			switch int(tm) {
			case 1:
				triggerMode = "MANUAL"
			case 2:
				triggerMode = "SCHEDULE"
			case 3:
				triggerMode = "PUSH"
			case 5:
				triggerMode = "PIPELINE"
			case 6:
				triggerMode = "WEBHOOK"
			default:
				triggerMode = fmt.Sprintf("UNKNOWN(%d)", int(tm))
			}
		}

		pipelineRun := PipelineRun{
			RunID:       runID,
			PipelineID:  pipelineID,
			Status:      getStringField(runMap, "status"),
			StartTime:   startTime,
			FinishTime:  finishTime,
			TriggerMode: triggerMode,
		}

		if pipelineRun.RunID != "" {
			pipelineRuns = append(pipelineRuns, pipelineRun)
		}
	}

	// Parse pagination headers
	totalPages := 1
	totalRuns := len(pipelineRuns)

	if tp, err := strconv.Atoi(resp.Header.Get("x-total-pages")); err == nil && tp > 0 {
		totalPages = tp
	}
	if tr, err := strconv.Atoi(resp.Header.Get("x-total")); err == nil && tr > 0 {
		totalRuns = tr
	}

	return &PipelineRunsResult{
		Runs:        pipelineRuns,
		CurrentPage: page,
		TotalPages:  totalPages,
		PerPage:     perPage,
		TotalRuns:   totalRuns,
		HasMore:     page < totalPages,
	}, nil
}

// ListPipelineRuns retrieves a list of runs for a specific pipeline.
// This fetches all pages.
func (c *Client) ListPipelineRuns(organizationId string, pipelineId string) ([]PipelineRun, error) {
	if organizationId == "" {
		return nil, fmt.Errorf("organizationId is required for ListPipelineRuns")
	}
	if pipelineId == "" {
		return nil, fmt.Errorf("pipelineId is required for ListPipelineRuns")
	}

	// Use different methods based on authentication type
	if c.useToken {
		return c.listPipelineRunsWithToken(organizationId, pipelineId)
	}

	// TODO: Implement SDK-based method for AccessKey authentication
	return nil, fmt.Errorf("ListPipelineRuns with AccessKey authentication not implemented yet")
}

// listPipelineRunsWithToken retrieves pipeline runs using personal access token authentication
func (c *Client) listPipelineRunsWithToken(organizationId, pipelineId string) ([]PipelineRun, error) {
	// Use the official ListPipelineRuns API endpoint
	// Based on: https://help.aliyun.com/zh/yunxiao/developer-reference/listpipelineruns
	// API endpoint: GET https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelines/{pipelineId}/runs
	officialPath := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s/runs", organizationId, pipelineId)

	// Fetch all pages of pipeline runs
	var allRuns []PipelineRun
	page := 1
	perPage := 30

	for {
		path := fmt.Sprintf("%s?page=%d&perPage=%d", officialPath, page, perPage)
		runs, hasMore, err := c.fetchPipelineRunsPage(path)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch pipeline runs page %d: %w", page, err)
		}

		allRuns = append(allRuns, runs...)

		// Check if we should continue to next page
		if !hasMore || len(runs) < perPage {
			break
		}
		page++
	}

	return allRuns, nil
}

// fetchPipelineRunsPage fetches a single page of pipeline runs and returns whether there are more pages
func (c *Client) fetchPipelineRunsPage(path string) ([]PipelineRun, bool, error) {
	// Make the request and get raw response
	url := fmt.Sprintf("https://%s%s", c.endpoint, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-yunxiao-token", c.personalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read response body: %w", err)
	}

	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("Trying pipeline runs endpoint: %s", url)
		debugLogger.Printf("Response Status: %d", resp.StatusCode)
		debugLogger.Printf("Response Body: %.500s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// According to API documentation, response is a direct array
	var runItems []map[string]interface{}
	if err := json.Unmarshal(respBody, &runItems); err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal response as array: %w. Response body: %.500s", err, string(respBody))
	}

	var pipelineRuns []PipelineRun

	// Parse each run item according to API documentation
	for _, runMap := range runItems {
		var startTime, finishTime time.Time

		// API returns timestamps as integers (milliseconds)
		if st, ok := runMap["startTime"].(float64); ok && st > 0 {
			startTime = time.Unix(int64(st)/1000, 0)
		}
		if ft, ok := runMap["endTime"].(float64); ok && ft > 0 { // Note: API uses "endTime" not "finishTime"
			finishTime = time.Unix(int64(ft)/1000, 0)
		}

		// Extract run ID from pipelineRunId field
		var runID string
		if id, ok := runMap["pipelineRunId"].(string); ok {
			runID = id
		} else if id, ok := runMap["pipelineRunId"].(float64); ok {
			runID = fmt.Sprintf("%.0f", id)
		}

		// Extract pipeline ID
		var pipelineID string
		if pid, ok := runMap["pipelineId"].(string); ok {
			pipelineID = pid
		} else if pid, ok := runMap["pipelineId"].(float64); ok {
			pipelineID = fmt.Sprintf("%.0f", pid)
		}

		// Map trigger mode from integer to string
		var triggerMode string
		if tm, ok := runMap["triggerMode"].(float64); ok {
			switch int(tm) {
			case 1:
				triggerMode = "MANUAL"
			case 2:
				triggerMode = "SCHEDULE"
			case 3:
				triggerMode = "PUSH"
			case 5:
				triggerMode = "PIPELINE"
			case 6:
				triggerMode = "WEBHOOK"
			default:
				triggerMode = fmt.Sprintf("UNKNOWN(%d)", int(tm))
			}
		}

		pipelineRun := PipelineRun{
			RunID:       runID,
			PipelineID:  pipelineID,
			Status:      getStringField(runMap, "status"),
			StartTime:   startTime,
			FinishTime:  finishTime,
			TriggerMode: triggerMode,
		}

		if pipelineRun.RunID != "" {
			pipelineRuns = append(pipelineRuns, pipelineRun)
		}
	}

	// Check pagination headers to determine if there are more pages
	totalPagesHeader := resp.Header.Get("x-total-pages")
	currentPageHeader := resp.Header.Get("x-page")
	hasMore := false

	if totalPagesHeader != "" && currentPageHeader != "" {
		// Use header information for pagination
		hasMore = currentPageHeader != totalPagesHeader
	}

	return pipelineRuns, hasMore, nil
}

// ListPipelineGroups retrieves a list of pipeline groups (projects) for an organization.
func (c *Client) ListPipelineGroups(organizationId string) ([]PipelineGroup, error) {
	if organizationId == "" {
		return nil, fmt.Errorf("organizationId is required for ListPipelineGroups")
	}

	// Use different methods based on authentication type
	if c.useToken {
		return c.listPipelineGroupsWithToken(organizationId)
	}

	// Use SDK for AccessKey authentication
	request := devops_rdc.CreateListDevopsProjectsRequest()
	request.Scheme = "https"
	request.OrgId = organizationId
	// request.PageSize = "100" // Example: Add pagination if needed and supported

	response, err := c.sdkClient.ListDevopsProjects(request)
	if err != nil {
		return nil, fmt.Errorf("failed to list devops projects (pipeline groups): %w", err)
	}

	if !response.Successful { // Note: Field name is `Successful` not `Success`
		return nil, fmt.Errorf("API error listing devops projects: %s (ErrorCode: %s)", response.ErrorMsg, response.ErrorCode)
	}

	var groups []PipelineGroup

	// The response.Object is of type devops_rdc.ObjectInListDevopsProjects
	// We need to determine its structure. Assuming it has a field like 'Result' or 'Projects'
	// which is a slice of project details.
	// For now, let's try to access a field named 'Result' and expect it to be []interface{}.
	// This is a common pattern. If this fails, we'll need to find the actual field name.
	// The Go SDK doesn't provide the nested struct definitions directly in the viewed file for `ObjectInListDevopsProjects`.
	// We'll have to rely on reflection or common patterns.
	// A safer way if the exact structure is unknown is to marshal response.Object to JSON and then unmarshal to a known map.
	// However, let's try direct access with an assumption.
	// Assume response.Object has a field `Result` which is a slice of maps.
	// This part is speculative due to missing definition of ObjectInListDevopsProjects.
	//
	// If `response.Object` itself is the list or contains the list directly, this will need adjustment.
	// The SDK source for `list_devops_projects.go` defines:
	// type ListDevopsProjectsResponse struct { ... Object ObjectInListDevopsProjects ... }
	// The type `ObjectInListDevopsProjects` is not defined in that specific file.
	//
	// Let's assume `ObjectInListDevopsProjects` has a field `Projects` (a common name)
	// or perhaps `Result`. Trying `Projects` first.
	// If `response.Object.Projects` exists and is a slice:
	// We cannot directly access `response.Object.Projects` without knowing the type `ObjectInListDevopsProjects`.
	//
	// A common way SDKs handle this is that `response.Object` might be a struct, and `response.Object.Projects` is the field.
	// Or `response.Object` is a map itself.
	// Given `Object       map[string]interface{} `json:"Object" xml:"Object"` in ListPipelinesResponse,
	// and `Object     ObjectInListDevopsProjects `json:"Object" xml:"Object"` in ListDevopsProjectsResponse,
	// it's likely ObjectInListDevopsProjects is a struct, not a map.
	//
	// The SDK code generator usually creates structs for these.
	// Let's try to access common field names like "Projects" or "Items" or "List" from response.Object
	// This requires knowing the structure of `ObjectInListDevopsProjects`.
	//
	// If we assume it's similar to ListPipelines and the actual data is in a map *within* Object,
	// but `Object` here is a specific struct type, not `map[string]interface{}`.
	// This means `Object` itself should have fields.
	// Let's try to find the definition of `ObjectInListDevopsProjects` by searching the SDK repo or making an educated guess.
	// A common structure is `TotalCount` and `Items []ActualItem`.
	//
	// Given the subtask asks to report on the structure, if this fails, the error will be informative.
	// For now, to make it compilable, I will assume `response.Object` has a field `Result` which is a slice.
	// This will likely fail at runtime if `Result` is not the field, or not a slice.
	// The Go way would be to check the actual generated SDK code.
	//
	// A more robust approach without full introspection tools here:
	// Assume `response.Object` has a field `Projects` which is a slice of structs,
	// and each struct has `ProjectId` and `Name`.
	// This still requires knowing the exact field name (`Projects`) and struct fields.
	//
	// Let's try to get the raw JSON of `response.Object` and unmarshal it.
	// This is a workaround for not having the exact struct definition.
	// Note: This is not ideal but can work if the structure is simple JSON.

	// For now, returning an error, as the structure of `ObjectInListDevopsProjects` is unknown.
	// The next step would be to inspect the SDK's generated code for this type.
	// If `ObjectInListDevopsProjects` has a field like `Items` or `Projects` of type `[]struct { ProjectId string; Name string; ...}`
	// then that would be used.
	// Example (pseudo-code, assuming `response.Object.Result` is `[]map[string]interface{}` which is a common dynamic way):
	/*
	   dataBytes, err := json.Marshal(response.Object)
	   if err != nil {
	       return nil, fmt.Errorf("failed to marshal response.Object: %w", err)
	   }

	   var tempObj struct {
	       // Try common field names for lists. "Result", "Items", "Projects", "List"
	       Result []map[string]interface{} `json:"Result"` // Or "Projects", "Items", etc.
	   }
	   if err := json.Unmarshal(dataBytes, &tempObj); err != nil {
	       return nil, fmt.Errorf("failed to unmarshal response.Object into tempObj: %w", err)
	   }

	   if tempObj.Result == nil {
	       // Try another key if Result was not found, e.g. "Projects"
	       // This indicates the assumed key "Result" was wrong.
	        return nil, fmt.Errorf("no 'Result' field found in unmarshalled response.Object, or it's null. Raw Object: %s", string(dataBytes))
	   }

	   for _, itemMap := range tempObj.Result {
	       groupID := getStringField(itemMap, "ProjectId") // Or "Id"
	       name := getStringField(itemMap, "Name")
	       if groupID != "" && name != "" {
	           groups = append(groups, PipelineGroup{
	               GroupID: groupID,
	               Name:    name,
	           })
	       }
	   }
	*/
	// The above JSON marshalling/unmarshalling is a robust way to explore unknown structs.
	// However, to proceed with the subtask, I need to make a direct assumption or state that it's blocked by unknown struct.
	// The subtask asks to report on the structure. The current structure is `devops_rdc.ObjectInListDevopsProjects`.
	// The fields within this struct are unknown.

	// Let's assume, based on common SDK patterns, that `ObjectInListDevopsProjects` might have a field named `Projects`.
	// And this field is a slice of structs, each having `ProjectId` and `Name`.
	// This is a strong assumption.
	// To make this compile, I would need the definition of `ObjectInListDevopsProjects`.
	// Since I don't have it, I cannot write the direct field access code that compiles.

	// Reporting: The API call used is `ListDevopsProjects`.
	// The response structure for the list of projects is within `response.Object` of type `devops_rdc.ObjectInListDevopsProjects`.
	// The internal structure of `devops_rdc.ObjectInListDevopsProjects` is not defined in the viewed SDK files.
	// To handle this, we marshal `response.Object` to JSON and then unmarshal it into a map[string]interface{}
	// to dynamically inspect its fields.

	dataBytes, err := json.Marshal(response.Object)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response.Object for dynamic parsing: %w. API Response: Successful=%v, ErrorCode=%s, ErrorMsg=%s", err, response.Successful, response.ErrorCode, response.ErrorMsg)
	}

	var objectMap map[string]interface{}
	if err := json.Unmarshal(dataBytes, &objectMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response.Object into map: %w. Raw Object JSON: %s", err, string(dataBytes))
	}

	// Now, try to find the list of projects within objectMap.
	// Common keys for lists: "Projects", "Items", "List", "Result", "Data"
	possibleKeys := []string{"Projects", "Result", "Items", "List", "Data"}
	var projectListInterfaces []interface{}
	foundListKey := ""

	for _, key := range possibleKeys {
		if listData, ok := objectMap[key].([]interface{}); ok {
			projectListInterfaces = listData
			foundListKey = key
			break
		}
	}

	if projectListInterfaces == nil {
		return nil, fmt.Errorf("could not find a known key ('%v') for project list in response.Object map. Available keys: %v. Raw Object JSON: %s", possibleKeys, getMapKeys(objectMap), string(dataBytes))
	}

	if len(projectListInterfaces) == 0 {
		return groups, nil // No projects found, but the call was successful
	}

	for _, projectInterface := range projectListInterfaces {
		projectMap, ok := projectInterface.(map[string]interface{})
		if !ok {
			// Log or skip malformed item
			// fmt.Fprintf(os.Stderr, "Warning: project item is not a map[string]interface{}: %T\n", projectInterface)
			continue
		}

		var group PipelineGroup
		// Try common keys for ID: "ProjectId", "Id", "ID"
		// Try common keys for Name: "Name", "ProjectName"

		if idVal, ok := projectMap["ProjectId"].(string); ok {
			group.GroupID = idVal
		} else if idVal, ok := projectMap["Id"].(string); ok {
			group.GroupID = idVal
		} else if idVal, ok := projectMap["ID"].(string); ok {
			group.GroupID = idVal
		} else if idValFloat, ok := projectMap["ProjectId"].(float64); ok { // Sometimes numbers come as float64
			group.GroupID = fmt.Sprintf("%.0f", idValFloat)
		} else if idValFloat, ok := projectMap["Id"].(float64); ok {
			group.GroupID = fmt.Sprintf("%.0f", idValFloat)
		} else if idValFloat, ok := projectMap["ID"].(float64); ok {
			group.GroupID = fmt.Sprintf("%.0f", idValFloat)
		}

		if nameVal, ok := projectMap["Name"].(string); ok {
			group.Name = nameVal
		} else if nameVal, ok := projectMap["ProjectName"].(string); ok {
			group.Name = nameVal
		}

		if group.GroupID != "" && group.Name != "" {
			groups = append(groups, group)
		} else {
			// Could log a warning if a project-like map was found but key fields were missing/empty
			// fmt.Fprintf(os.Stderr, "Warning: Found project map but GroupID or Name is missing/empty: %+v\n", projectMap)
		}
	}

	if len(groups) == 0 && len(projectListInterfaces) > 0 {
		// This means we found project items, but couldn't extract ID/Name from any of them.
		return nil, fmt.Errorf("found %d project items under key '%s', but failed to extract GroupID/Name from any. Check API response structure and expected keys. First item: %+v", len(projectListInterfaces), foundListKey, projectListInterfaces[0])
	}

	return groups, nil
}

// makeTokenRequest makes an HTTP request using personal access token authentication
func (c *Client) makeTokenRequest(method, path string, body interface{}) (map[string]interface{}, error) {
	if !c.useToken {
		return nil, fmt.Errorf("client not configured for token-based requests")
	}

	url := fmt.Sprintf("https://%s%s", c.endpoint, path)

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set correct authentication header for Aliyun DevOps API
	// Based on official documentation: use x-yunxiao-token header
	req.Header.Set("x-yunxiao-token", c.personalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("Request URL: %s", url)
		debugLogger.Printf("Request Method: %s", method)
		debugLogger.Printf("Request Headers: %v", req.Header)
		debugLogger.Printf("Response Status: %d", resp.StatusCode)
		debugLogger.Printf("Response Headers: %v", resp.Header)
		debugLogger.Printf("Response Body (first 1000 chars): %.1000s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Check if response looks like HTML (common when authentication fails)
	if len(respBody) > 0 && respBody[0] == '<' {
		return nil, fmt.Errorf("received HTML response instead of JSON (status %d). This usually indicates authentication failure or wrong endpoint. Response preview: %.200s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w. Response body: %.500s", err, string(respBody))
	}

	return result, nil
}

// listPipelinesWithToken retrieves pipelines using personal access token authentication
func (c *Client) listPipelinesWithToken(organizationId string) ([]Pipeline, error) {
	return c.listPipelinesWithTokenAndStatus(organizationId, nil)
}

func (c *Client) listPipelinesWithTokenAndStatus(organizationId string, statusList []string) ([]Pipeline, error) {
	// Based on official Aliyun DevOps API documentation:
	// https://help.aliyun.com/zh/yunxiao/developer-reference/listpipelines-get-a-list-of-pipelines
	// GET https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelines

	var allPipelines []Pipeline
	page := 1
	perPage := 30 // Maximum per page according to API docs

	for {
		// Build query parameters
		queryParams := fmt.Sprintf("page=%d&perPage=%d", page, perPage)

		// Add status filtering if provided
		if len(statusList) > 0 {
			statusParam := strings.Join(statusList, ",")
			queryParams += fmt.Sprintf("&statusList=%s", statusParam)
		}

		path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines?%s", organizationId, queryParams)

		// Make the request and get raw response
		url := fmt.Sprintf("https://%s%s", c.endpoint, path)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("x-yunxiao-token", c.personalAccessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if os.Getenv("FLOWT_DEBUG") == "1" {
			debugLogger.Printf("Request URL: %s", url)
			debugLogger.Printf("Response Status: %d", resp.StatusCode)
			debugLogger.Printf("Response Headers: %v", resp.Header)
			debugLogger.Printf("Response Body (first 1000 chars): %.1000s", string(respBody))
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		// According to API docs, response is a direct array
		var pipelineItems []map[string]interface{}
		if err := json.Unmarshal(respBody, &pipelineItems); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response as array: %w. Response body: %.500s", err, string(respBody))
		}

		// If no items found, we've reached the end
		if len(pipelineItems) == 0 {
			break
		}

		// Check pagination headers to determine if there are more pages
		totalPagesHeader := resp.Header.Get("x-total-pages")
		currentPageHeader := resp.Header.Get("x-page")

		if os.Getenv("FLOWT_DEBUG") == "1" {
			debugLogger.Printf("Pagination info - Current page: %s, Total pages: %s, Items in this page: %d", currentPageHeader, totalPagesHeader, len(pipelineItems))
		}

		// Parse each pipeline item
		for _, pipelineMap := range pipelineItems {
			if os.Getenv("FLOWT_DEBUG") == "1" {
				debugLogger.Printf("Processing pipeline item: %+v", pipelineMap)
			}

			var createTime, updateTime, lastRunTime time.Time
			if ct, ok := pipelineMap["createTime"].(float64); ok && ct > 0 {
				createTime = time.Unix(int64(ct)/1000, 0)
			}
			if ut, ok := pipelineMap["updateTime"].(float64); ok && ut > 0 {
				updateTime = time.Unix(int64(ut)/1000, 0)
			}

			// Extract pipeline ID - it might be string or number
			var pipelineID string
			if id, ok := pipelineMap["id"].(string); ok {
				pipelineID = id
			} else if id, ok := pipelineMap["id"].(float64); ok {
				pipelineID = fmt.Sprintf("%.0f", id)
			} else if id, ok := pipelineMap["pipelineId"].(string); ok {
				pipelineID = id
			} else if id, ok := pipelineMap["pipelineId"].(float64); ok {
				pipelineID = fmt.Sprintf("%.0f", id)
			}

			if os.Getenv("FLOWT_DEBUG") == "1" {
				debugLogger.Printf("Extracted pipeline ID: '%s'", pipelineID)
			}

			// Extract creator information
			var creator, creatorName string
			if creatorObj, ok := pipelineMap["creator"].(map[string]interface{}); ok {
				creator = getStringField(creatorObj, "id")
				creatorName = getStringField(creatorObj, "username")
				if creatorName == "" {
					creatorName = getStringField(creatorObj, "name")
				}
				if creatorName == "" {
					creatorName = getStringField(creatorObj, "displayName")
				}
			}
			if creator == "" {
				creator = getStringField(pipelineMap, "creatorAccountId")
			}
			if creatorName == "" {
				creatorName = creator // Fallback to ID if name not available
			}

			// Extract last run information
			var lastRunStatus string
			if lastRunObj, ok := pipelineMap["lastRun"].(map[string]interface{}); ok {
				lastRunStatus = getStringField(lastRunObj, "status")
				if lrt, ok := lastRunObj["finishTime"].(float64); ok && lrt > 0 {
					lastRunTime = time.Unix(int64(lrt)/1000, 0)
				} else if lrt, ok := lastRunObj["startTime"].(float64); ok && lrt > 0 {
					lastRunTime = time.Unix(int64(lrt)/1000, 0)
				}
			}
			// Try alternative field names for last run status
			if lastRunStatus == "" {
				lastRunStatus = getStringField(pipelineMap, "lastRunStatus")
			}
			if lastRunStatus == "" {
				lastRunStatus = getStringField(pipelineMap, "latestRunStatus")
			}

			// Extract pipeline status - handle cases where status field might be missing
			pipelineStatus := getStringField(pipelineMap, "status")
			// If status is missing but we have statusList filter, infer the status
			if pipelineStatus == "" && len(statusList) > 0 {
				// When filtering by status, the returned pipelines should match the filter
				// Use the first status from the filter as a reasonable default
				pipelineStatus = statusList[0]
			}
			// If still no status, use lastRunStatus as fallback
			if pipelineStatus == "" && lastRunStatus != "" {
				pipelineStatus = lastRunStatus
			}

			pipelineName := getStringField(pipelineMap, "name")

			if os.Getenv("FLOWT_DEBUG") == "1" {
				debugLogger.Printf("Extracted pipeline name: '%s', status: '%s'", pipelineName, pipelineStatus)
			}

			pipeline := Pipeline{
				PipelineID:    pipelineID,
				Name:          pipelineName,
				Status:        pipelineStatus,
				LastRunStatus: lastRunStatus,
				LastRunTime:   lastRunTime,
				Creator:       creator,
				CreatorName:   creatorName,
				Modifier:      getStringField(pipelineMap, "modifierAccountId"),
				CreateTime:    createTime,
				UpdateTime:    updateTime,
			}

			// Only include pipelines that have both ID and name
			if pipeline.PipelineID != "" && pipeline.Name != "" {
				allPipelines = append(allPipelines, pipeline)
				if os.Getenv("FLOWT_DEBUG") == "1" {
					debugLogger.Printf("Added pipeline: ID='%s', Name='%s', Status='%s'", pipeline.PipelineID, pipeline.Name, pipeline.Status)
				}
			} else {
				if os.Getenv("FLOWT_DEBUG") == "1" {
					debugLogger.Printf("Skipped pipeline due to missing ID or name: ID='%s', Name='%s'", pipeline.PipelineID, pipeline.Name)
				}
			}
		}

		// Check if there are more pages using response headers
		totalPagesStr := resp.Header.Get("x-total-pages")
		currentPageStr := resp.Header.Get("x-page")

		if totalPagesStr != "" && currentPageStr != "" {
			// Use header information for pagination
			if currentPageStr == totalPagesStr {
				// We've reached the last page
				break
			}
		} else {
			// Fallback: if we got fewer items than perPage, we've reached the end
			if len(pipelineItems) < perPage {
				break
			}
		}

		page++
	}

	return allPipelines, nil
}

// getPipelineRunWithToken retrieves pipeline run details using personal access token authentication
func (c *Client) getPipelineRunWithToken(organizationId, pipelineIdStr, runIdStr string) (*PipelineRun, error) {
	// Based on Aliyun DevOps API pattern, pipeline run details might follow similar structure
	// This needs to be updated with the correct API endpoint for getting pipeline run details
	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s/runs/%s", organizationId, pipelineIdStr, runIdStr)

	response, err := c.makeTokenRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline run with token: %w", err)
	}

	// Check if the response has a success indicator
	if success, ok := response["success"].(bool); ok && !success {
		errorMsg, _ := response["errorMessage"].(string)
		errorCode, _ := response["errorCode"].(string)
		return nil, fmt.Errorf("API error: %s (ErrorCode: %s)", errorMsg, errorCode)
	}

	// According to the official API documentation, the response directly contains the pipeline run data
	// No need to extract from "data" or "result" fields
	runMap := response

	// Parse timestamps according to API documentation (timestamps are in milliseconds)
	var startTime, finishTime time.Time
	if createTime, ok := runMap["createTime"].(float64); ok && createTime > 0 {
		startTime = time.Unix(int64(createTime)/1000, 0)
	}
	if updateTime, ok := runMap["updateTime"].(float64); ok && updateTime > 0 {
		finishTime = time.Unix(int64(updateTime)/1000, 0)
	}

	// Extract run ID according to API documentation
	var runID string
	if id, ok := runMap["pipelineRunId"].(float64); ok {
		runID = fmt.Sprintf("%.0f", id)
	} else if id, ok := runMap["pipelineRunId"].(string); ok {
		runID = id
	} else {
		runID = runIdStr // Fallback to input
	}

	// Parse trigger mode according to API documentation (it's an integer)
	var triggerMode string
	if tm, ok := runMap["triggerMode"].(float64); ok {
		switch int(tm) {
		case 1:
			triggerMode = "MANUAL"
		case 2:
			triggerMode = "SCHEDULE"
		case 3:
			triggerMode = "PUSH"
		case 5:
			triggerMode = "PIPELINE"
		case 6:
			triggerMode = "WEBHOOK"
		default:
			triggerMode = fmt.Sprintf("UNKNOWN(%d)", int(tm))
		}
	} else {
		triggerMode = getStringField(runMap, "triggerMode")
	}

	pipelineRun := &PipelineRun{
		RunID:       runID,
		PipelineID:  pipelineIdStr,
		Status:      getStringField(runMap, "status"),
		TriggerMode: triggerMode,
		StartTime:   startTime,
		FinishTime:  finishTime,
	}

	return pipelineRun, nil
}

// ListPipelineJobHistorys retrieves pipeline job execution history using the correct API endpoint
// Based on: https://help.aliyun.com/zh/yunxiao/developer-reference/listpipelinejobhistorys
func (c *Client) ListPipelineJobHistorys(organizationId, pipelineId, category, identifier string, page, perPage int) ([]PipelineRun, error) {
	if !c.useToken {
		return nil, fmt.Errorf("ListPipelineJobHistorys only supports token-based authentication")
	}

	if organizationId == "" {
		return nil, fmt.Errorf("organizationId is required")
	}
	if pipelineId == "" {
		return nil, fmt.Errorf("pipelineId is required")
	}
	if category == "" {
		category = "DEPLOY" // Default category as per documentation
	}
	if identifier == "" {
		return nil, fmt.Errorf("identifier is required")
	}
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 || perPage > 30 {
		perPage = 10 // Default per page as per documentation
	}

	// Construct the API path according to documentation
	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/getComponentsWithoutButtons?pipelineId=%s&category=%s&identifier=%s&perPage=%d&page=%d",
		organizationId, pipelineId, category, identifier, perPage, page)

	// Make the request
	url := fmt.Sprintf("https://%s%s", c.endpoint, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-yunxiao-token", c.personalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("ListPipelineJobHistorys URL: %s", url)
		debugLogger.Printf("Response Status: %d", resp.StatusCode)
		debugLogger.Printf("Response Headers: %v", resp.Header)
		debugLogger.Printf("Response Body: %.1000s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// According to documentation, response is a direct array
	var jobHistoryItems []map[string]interface{}
	if err := json.Unmarshal(respBody, &jobHistoryItems); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response as array: %w. Response body: %.500s", err, string(respBody))
	}

	var pipelineRuns []PipelineRun

	// Parse each job history item according to documentation structure
	for _, jobMap := range jobHistoryItems {
		var startTime, finishTime time.Time

		// The API doesn't provide start/finish times directly, so we'll use executeNumber as a proxy
		executeNumber := int(getNumberField(jobMap, "executeNumber"))

		// Extract run ID from pipelineRunId
		var runID string
		if id, ok := jobMap["pipelineRunId"].(string); ok {
			runID = id
		} else if id, ok := jobMap["pipelineRunId"].(float64); ok {
			runID = fmt.Sprintf("%.0f", id)
		}

		// Extract job ID as backup for run ID
		if runID == "" {
			if id, ok := jobMap["jobId"].(string); ok {
				runID = id
			} else if id, ok := jobMap["jobId"].(float64); ok {
				runID = fmt.Sprintf("%.0f", id)
			}
		}

		pipelineRun := PipelineRun{
			RunID:       runID,
			PipelineID:  pipelineId,
			Status:      getStringField(jobMap, "status"),
			StartTime:   startTime,
			FinishTime:  finishTime,
			TriggerMode: fmt.Sprintf("Execute #%d", executeNumber), // Use execute number as trigger info
		}

		if pipelineRun.RunID != "" {
			pipelineRuns = append(pipelineRuns, pipelineRun)
		}
	}

	return pipelineRuns, nil
}

// GetVMDeployOrder retrieves VM deployment order details
// Based on: https://help.aliyun.com/zh/yunxiao/developer-reference/getvmdeployorder
func (c *Client) GetVMDeployOrder(organizationId, pipelineId, deployOrderId string) (*VMDeployOrder, error) {
	if !c.useToken {
		return nil, fmt.Errorf("GetVMDeployOrder only supports token-based authentication")
	}

	if organizationId == "" || pipelineId == "" || deployOrderId == "" {
		return nil, fmt.Errorf("organizationId, pipelineId, and deployOrderId are required")
	}

	// API endpoint: GET https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelines/{pipelineId}/deploy/{deployOrderId}
	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s/deploy/%s", organizationId, pipelineId, deployOrderId)
	url := fmt.Sprintf("https://%s%s", c.endpoint, path)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-yunxiao-token", c.personalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("GetVMDeployOrder URL: %s", url)
		debugLogger.Printf("Response Status: %d", resp.StatusCode)
		debugLogger.Printf("Response Body: %.1000s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var responseData map[string]interface{}
	if err := json.Unmarshal(respBody, &responseData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Parse the response according to API documentation
	deployOrder := &VMDeployOrder{}

	if deployOrderId, ok := responseData["deployOrderId"].(float64); ok {
		deployOrder.DeployOrderId = int(deployOrderId)
	}
	if status, ok := responseData["status"].(string); ok {
		deployOrder.Status = status
	}
	if creator, ok := responseData["creator"].(string); ok {
		deployOrder.Creator = creator
	}
	if createTime, ok := responseData["createTime"].(float64); ok {
		deployOrder.CreateTime = int64(createTime)
	}
	if updateTime, ok := responseData["updateTime"].(float64); ok {
		deployOrder.UpdateTime = int64(updateTime)
	}
	if currentBatch, ok := responseData["currentBatch"].(float64); ok {
		deployOrder.CurrentBatch = int(currentBatch)
	}
	if totalBatch, ok := responseData["totalBatch"].(float64); ok {
		deployOrder.TotalBatch = int(totalBatch)
	}

	// Parse deployMachineInfo
	if deployMachineInfoData, ok := responseData["deployMachineInfo"].(map[string]interface{}); ok {
		if batchNum, ok := deployMachineInfoData["batchNum"].(float64); ok {
			deployOrder.DeployMachineInfo.BatchNum = int(batchNum)
		}
		if hostGroupId, ok := deployMachineInfoData["hostGroupId"].(float64); ok {
			deployOrder.DeployMachineInfo.HostGroupId = int(hostGroupId)
		}

		// Parse deployMachines array
		if deployMachinesData, ok := deployMachineInfoData["deployMachines"].([]interface{}); ok {
			for _, machineItem := range deployMachinesData {
				if machineMap, ok := machineItem.(map[string]interface{}); ok {
					machine := VMDeployMachine{}
					if ip, ok := machineMap["ip"].(string); ok {
						machine.IP = ip
					}
					if machineSn, ok := machineMap["machineSn"].(string); ok {
						machine.MachineSn = machineSn
					}
					if status, ok := machineMap["status"].(string); ok {
						machine.Status = status
					}
					if clientStatus, ok := machineMap["clientStatus"].(string); ok {
						machine.ClientStatus = clientStatus
					}
					if batchNum, ok := machineMap["batchNum"].(float64); ok {
						machine.BatchNum = int(batchNum)
					}
					if createTime, ok := machineMap["createTime"].(float64); ok {
						machine.CreateTime = int64(createTime)
					}
					if updateTime, ok := machineMap["updateTime"].(float64); ok {
						machine.UpdateTime = int64(updateTime)
					}
					deployOrder.DeployMachineInfo.DeployMachines = append(deployOrder.DeployMachineInfo.DeployMachines, machine)
				}
			}
		}
	}

	return deployOrder, nil
}

// GetVMDeployMachineLog retrieves deployment log for a specific machine
// Based on: https://help.aliyun.com/zh/yunxiao/developer-reference/getvmdeploymachinelog
func (c *Client) GetVMDeployMachineLog(organizationId, pipelineId, deployOrderId, machineSn string) (*VMDeployMachineLog, error) {
	if !c.useToken {
		return nil, fmt.Errorf("GetVMDeployMachineLog only supports token-based authentication")
	}

	if organizationId == "" || pipelineId == "" || deployOrderId == "" || machineSn == "" {
		return nil, fmt.Errorf("organizationId, pipelineId, deployOrderId, and machineSn are required")
	}

	// API endpoint: GET https://{domain}/oapi/v1/flow/organizations/{organizationId}/pipelines/{pipelineId}/deploy/{deployOrderId}/machine/{machineSn}/log
	path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines/%s/deploy/%s/machine/%s/log", organizationId, pipelineId, deployOrderId, machineSn)
	url := fmt.Sprintf("https://%s%s", c.endpoint, path)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-yunxiao-token", c.personalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if os.Getenv("FLOWT_DEBUG") == "1" {
		debugLogger.Printf("GetVMDeployMachineLog URL: %s", url)
		debugLogger.Printf("Response Status: %d", resp.StatusCode)
		debugLogger.Printf("Response Body: %.1000s", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	var responseData map[string]interface{}
	if err := json.Unmarshal(respBody, &responseData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Parse the response according to API documentation
	machineLog := &VMDeployMachineLog{}

	if aliyunRegion, ok := responseData["aliyunRegion"].(string); ok {
		machineLog.AliyunRegion = aliyunRegion
	}
	if deployBeginTime, ok := responseData["deployBeginTime"].(string); ok {
		machineLog.DeployBeginTime = deployBeginTime
	}
	if deployEndTime, ok := responseData["deployEndTime"].(string); ok {
		machineLog.DeployEndTime = deployEndTime
	}
	if deployLog, ok := responseData["deployLog"].(string); ok {
		machineLog.DeployLog = deployLog
	}
	if deployLogPath, ok := responseData["deployLogPath"].(string); ok {
		machineLog.DeployLogPath = deployLogPath
	}

	return machineLog, nil
}

// PipelinePageCallback is called for each page of pipelines loaded
type PipelinePageCallback func(pipelines []Pipeline, currentPage, totalPages int, isComplete bool) error

// ListPipelinesWithCallback loads pipelines page by page and calls the callback for each page
func (c *Client) ListPipelinesWithCallback(organizationId string, callback PipelinePageCallback) error {
	return c.listPipelinesWithCallbackAndStatus(organizationId, nil, callback)
}

// ListPipelinesWithStatusAndCallback loads pipelines with status filter page by page and calls the callback for each page
func (c *Client) ListPipelinesWithStatusAndCallback(organizationId string, statusList []string, callback PipelinePageCallback) error {
	return c.listPipelinesWithCallbackAndStatus(organizationId, statusList, callback)
}

// listPipelinesWithCallbackAndStatus implements the core pagination logic with callback
func (c *Client) listPipelinesWithCallbackAndStatus(organizationId string, statusList []string, callback PipelinePageCallback) error {
	if c.useToken {
		return c.listPipelinesWithTokenAndCallback(organizationId, statusList, callback)
	}

	// For SDK-based authentication, fall back to loading all at once
	// This could be enhanced later if needed
	pipelines, err := c.ListPipelinesWithStatus(organizationId, statusList)
	if err != nil {
		return err
	}

	// Call callback with all pipelines as a single page
	return callback(pipelines, 1, 1, true)
}

// listPipelinesWithTokenAndCallback implements token-based pagination with callback
func (c *Client) listPipelinesWithTokenAndCallback(organizationId string, statusList []string, callback PipelinePageCallback) error {
	page := 1
	perPage := 30   // Maximum per page according to API docs
	totalPages := 1 // Will be updated from response headers

	for {
		// Build query parameters
		queryParams := fmt.Sprintf("page=%d&perPage=%d", page, perPage)

		// Add status filtering if provided
		if len(statusList) > 0 {
			statusParam := strings.Join(statusList, ",")
			queryParams += fmt.Sprintf("&statusList=%s", statusParam)
		}

		path := fmt.Sprintf("/oapi/v1/flow/organizations/%s/pipelines?%s", organizationId, queryParams)

		// Make the request and get raw response
		url := fmt.Sprintf("https://%s%s", c.endpoint, path)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("x-yunxiao-token", c.personalAccessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "flowt-aliyun-devops-client/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}

		if os.Getenv("FLOWT_DEBUG") == "1" {
			debugLogger.Printf("Request URL: %s", url)
			debugLogger.Printf("Response Status: %d", resp.StatusCode)
			debugLogger.Printf("Response Headers: %v", resp.Header)
			debugLogger.Printf("Response Body (first 1000 chars): %.1000s", string(respBody))
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
		}

		// According to API docs, response is a direct array
		var pipelineItems []map[string]interface{}
		if err := json.Unmarshal(respBody, &pipelineItems); err != nil {
			return fmt.Errorf("failed to unmarshal response as array: %w. Response body: %.500s", err, string(respBody))
		}

		// Update total pages from headers if available
		if totalPagesHeader := resp.Header.Get("x-total-pages"); totalPagesHeader != "" {
			if tp, err := strconv.Atoi(totalPagesHeader); err == nil && tp > 0 {
				totalPages = tp
			}
		}

		// If no items found, we've reached the end
		if len(pipelineItems) == 0 {
			// Call callback with empty slice to indicate completion
			return callback([]Pipeline{}, page, totalPages, true)
		}

		// Parse pipelines for this page
		var pagePipelines []Pipeline
		for _, pipelineMap := range pipelineItems {
			if os.Getenv("FLOWT_DEBUG") == "1" {
				debugLogger.Printf("Processing pipeline item: %+v", pipelineMap)
			}

			var createTime, updateTime, lastRunTime time.Time
			if ct, ok := pipelineMap["createTime"].(float64); ok && ct > 0 {
				createTime = time.Unix(int64(ct)/1000, 0)
			}
			if ut, ok := pipelineMap["updateTime"].(float64); ok && ut > 0 {
				updateTime = time.Unix(int64(ut)/1000, 0)
			}

			// Extract pipeline ID - it might be string or number
			var pipelineID string
			if id, ok := pipelineMap["id"].(string); ok {
				pipelineID = id
			} else if id, ok := pipelineMap["id"].(float64); ok {
				pipelineID = fmt.Sprintf("%.0f", id)
			} else if id, ok := pipelineMap["pipelineId"].(string); ok {
				pipelineID = id
			} else if id, ok := pipelineMap["pipelineId"].(float64); ok {
				pipelineID = fmt.Sprintf("%.0f", id)
			}

			if os.Getenv("FLOWT_DEBUG") == "1" {
				debugLogger.Printf("Extracted pipeline ID: '%s'", pipelineID)
			}

			// Extract creator information
			var creator, creatorName string
			if creatorObj, ok := pipelineMap["creator"].(map[string]interface{}); ok {
				creator = getStringField(creatorObj, "id")
				creatorName = getStringField(creatorObj, "username")
				if creatorName == "" {
					creatorName = getStringField(creatorObj, "name")
				}
				if creatorName == "" {
					creatorName = getStringField(creatorObj, "displayName")
				}
			}
			if creator == "" {
				creator = getStringField(pipelineMap, "creatorAccountId")
			}
			if creatorName == "" {
				creatorName = creator // Fallback to ID if name not available
			}

			// Extract last run information
			var lastRunStatus string
			if lastRunObj, ok := pipelineMap["lastRun"].(map[string]interface{}); ok {
				lastRunStatus = getStringField(lastRunObj, "status")
				if lrt, ok := lastRunObj["finishTime"].(float64); ok && lrt > 0 {
					lastRunTime = time.Unix(int64(lrt)/1000, 0)
				} else if lrt, ok := lastRunObj["startTime"].(float64); ok && lrt > 0 {
					lastRunTime = time.Unix(int64(lrt)/1000, 0)
				}
			}
			// Try alternative field names for last run status
			if lastRunStatus == "" {
				lastRunStatus = getStringField(pipelineMap, "lastRunStatus")
			}
			if lastRunStatus == "" {
				lastRunStatus = getStringField(pipelineMap, "latestRunStatus")
			}

			// Extract pipeline status - handle cases where status field might be missing
			pipelineStatus := getStringField(pipelineMap, "status")
			// If status is missing but we have statusList filter, infer the status
			if pipelineStatus == "" && len(statusList) > 0 {
				// When filtering by status, the returned pipelines should match the filter
				// Use the first status from the filter as a reasonable default
				pipelineStatus = statusList[0]
			}
			// If still no status, use lastRunStatus as fallback
			if pipelineStatus == "" && lastRunStatus != "" {
				pipelineStatus = lastRunStatus
			}

			pipelineName := getStringField(pipelineMap, "name")

			if os.Getenv("FLOWT_DEBUG") == "1" {
				debugLogger.Printf("Extracted pipeline name: '%s', status: '%s'", pipelineName, pipelineStatus)
			}

			pipeline := Pipeline{
				PipelineID:    pipelineID,
				Name:          pipelineName,
				Status:        pipelineStatus,
				LastRunStatus: lastRunStatus,
				LastRunTime:   lastRunTime,
				Creator:       creator,
				CreatorName:   creatorName,
				Modifier:      getStringField(pipelineMap, "modifierAccountId"),
				CreateTime:    createTime,
				UpdateTime:    updateTime,
			}

			// Only include pipelines that have both ID and name
			if pipeline.PipelineID != "" && pipeline.Name != "" {
				pagePipelines = append(pagePipelines, pipeline)
				if os.Getenv("FLOWT_DEBUG") == "1" {
					debugLogger.Printf("Added pipeline: ID='%s', Name='%s', Status='%s'", pipeline.PipelineID, pipeline.Name, pipeline.Status)
				}
			} else {
				if os.Getenv("FLOWT_DEBUG") == "1" {
					debugLogger.Printf("Skipped pipeline due to missing ID or name: ID='%s', Name='%s'", pipeline.PipelineID, pipeline.Name)
				}
			}
		}

		// Determine if this is the last page
		isLastPage := false

		// Check pagination headers
		totalPagesStr := resp.Header.Get("x-total-pages")
		currentPageStr := resp.Header.Get("x-page")

		if totalPagesStr != "" && currentPageStr != "" {
			// Use header information for pagination
			if currentPageStr == totalPagesStr {
				isLastPage = true
			}
		} else {
			// Fallback: if we got fewer items than perPage, we've reached the end
			if len(pipelineItems) < perPage {
				isLastPage = true
			}
		}

		// Call callback with this page's pipelines
		if err := callback(pagePipelines, page, totalPages, isLastPage); err != nil {
			return err
		}

		// Break if this was the last page
		if isLastPage {
			break
		}

		page++
	}

	return nil
}
