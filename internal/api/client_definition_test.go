package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func definitionTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClientWithToken("devops.cn-hangzhou.aliyuncs.com", "test-token")
	if err != nil {
		t.Fatalf("NewClientWithToken: %v", err)
	}
	c.baseURLOverride = srv.URL
	return c
}

func TestGetPipelineDefinition(t *testing.T) {
	c := definitionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/organizations/org1/pipelines/123") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-yunxiao-token") != "test-token" {
			t.Errorf("missing token header")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         123,
			"name":       "deploy-service",
			"type":       "PIPELINEASCODE",
			"updateTime": float64(1758000000000),
			"pipelineConfig": map[string]interface{}{
				"flow":     "stages:\n  build:\n",
				"settings": "{}",
			},
		})
	})

	def, err := c.GetPipelineDefinition("org1", "123")
	if err != nil {
		t.Fatalf("GetPipelineDefinition: %v", err)
	}
	if !def.IsYAMLMode {
		t.Error("expected IsYAMLMode=true for PIPELINEASCODE")
	}
	if def.Name != "deploy-service" {
		t.Errorf("Name = %q", def.Name)
	}
	if def.FlowYAML != "stages:\n  build:\n" {
		t.Errorf("FlowYAML = %q", def.FlowYAML)
	}
	if def.UpdateTime != 1758000000000 {
		t.Errorf("UpdateTime = %d", def.UpdateTime)
	}
}

func TestGetPipelineDefinitionClassicMode(t *testing.T) {
	c := definitionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "classic-pipe",
			"pipelineConfig": map[string]interface{}{
				"flow": "schema: tb\npipeline:\n",
			},
		})
	})

	def, err := c.GetPipelineDefinition("org1", "123")
	if err != nil {
		t.Fatalf("GetPipelineDefinition: %v", err)
	}
	if def.IsYAMLMode {
		t.Error("expected IsYAMLMode=false when type is null/absent")
	}
	if def.FlowYAML != "schema: tb\npipeline:\n" {
		t.Errorf("FlowYAML = %q", def.FlowYAML)
	}
}

func TestGetPipelineDefinitionRequiresToken(t *testing.T) {
	c := &Client{useToken: false}
	if _, err := c.GetPipelineDefinition("org1", "123"); err == nil {
		t.Fatal("expected error under AccessKey auth")
	} else if !strings.Contains(err.Error(), "personal access token") {
		t.Errorf("unexpected error: %v", err)
	}
	if err := c.UpdatePipelineYAML("org1", "123", "n", "c"); err == nil {
		t.Fatal("expected error under AccessKey auth")
	}
}

func TestGetPipelineDefinitionMissingFlow(t *testing.T) {
	c := definitionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "empty"})
	})
	if _, err := c.GetPipelineDefinition("org1", "123"); err == nil {
		t.Fatal("expected error when flow is missing")
	}
}

func TestUpdatePipelineYAML(t *testing.T) {
	c := definitionTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["name"] != "deploy-service" {
			t.Errorf("name = %q", body["name"])
		}
		if body["content"] != "stages: []\n" {
			t.Errorf("content = %q", body["content"])
		}
		json.NewEncoder(w).Encode(true)
	})

	if err := c.UpdatePipelineYAML("org1", "123", "deploy-service", "stages: []\n"); err != nil {
		t.Fatalf("UpdatePipelineYAML: %v", err)
	}
}
