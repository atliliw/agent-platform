// Package handler provides HTTP handlers for Gateway
package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agent-platform/pkg/config"
	pb "agent-platform/pkg/pb/agent"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

// SkillHandler handles skill CRUD requests, proxying to the agent service.
type SkillHandler struct {
	cfg         *config.Config
	agentClient pb.AgentServiceClient
	conn        *grpc.ClientConn
}

// NewSkillHandler creates a new skill handler with a gRPC connection to agent service.
func NewSkillHandler(cfg *config.Config) *SkillHandler {
	conn, err := grpc.Dial(cfg.Services.Agent, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	return &SkillHandler{
		cfg:         cfg,
		agentClient: pb.NewAgentServiceClient(conn),
		conn:        conn,
	}
}

// Close closes the gRPC connection.
func (h *SkillHandler) Close() {
	if h.conn != nil {
		h.conn.Close()
	}
}

// skillRequest is the JSON body for create/update skill requests.
type skillRequest struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Instructions string   `json:"instructions"`
	Tools        []string `json:"tools"`
	Tags         []string `json:"tags"`
	Status       string   `json:"status"`
}

func (r *skillRequest) toProto() *pb.Skill {
	return &pb.Skill{
		Id:           r.ID,
		Name:         r.Name,
		Description:  r.Description,
		Instructions: r.Instructions,
		Tools:        r.Tools,
		Tags:         r.Tags,
		Status:       r.Status,
	}
}

// CreateSkill handles POST /skills
func (h *SkillHandler) CreateSkill(c *gin.Context) {
	var req skillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.agentClient.CreateSkill(ctx, &pb.CreateSkillRequest{Skill: req.toProto()})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "data": gin.H{"skill": resp.Skill}})
}

// ListSkills handles GET /skills
func (h *SkillHandler) ListSkills(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.agentClient.ListSkills(ctx, &pb.ListSkillsRequest{})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"skills":     resp.Skills,
			"pagination": resp.Pagination,
		},
	})
}

// GetSkill handles GET /skills/:id
func (h *SkillHandler) GetSkill(c *gin.Context) {
	skillID := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.agentClient.GetSkill(ctx, &pb.GetSkillRequest{SkillId: skillID})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "data": gin.H{"skill": resp.Skill}})
}

// UpdateSkill handles PUT /skills/:id
func (h *SkillHandler) UpdateSkill(c *gin.Context) {
	skillID := c.Param("id")

	var req skillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request"})
		return
	}
	// Ensure the path ID wins (idempotent upsert target).
	req.ID = skillID

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.agentClient.UpdateSkill(ctx, &pb.UpdateSkillRequest{Skill: req.toProto()})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "data": gin.H{"skill": resp.Skill}})
}

// DeleteSkill handles DELETE /skills/:id
func (h *SkillHandler) DeleteSkill(c *gin.Context) {
	skillID := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.agentClient.DeleteSkill(ctx, &pb.DeleteSkillRequest{SkillId: skillID})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "success": resp.Success})
}

// skillYAML is the portable YAML representation of a skill for import/export.
// It mirrors agent.Skill's yaml tags so an exported file round-trips cleanly
// back through import. System-managed fields (version, timestamps) are
// intentionally omitted: import treats them as server-managed.
type skillYAML struct {
	ID           string   `yaml:"id,omitempty"`
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Instructions string   `yaml:"instructions"`
	Tools        []string `yaml:"tools,omitempty"`
	Tags         []string `yaml:"tags,omitempty"`
	Status       string   `yaml:"status,omitempty"`
}

// ExportSkill handles GET /skills/:id/export. Returns the skill as a portable
// YAML document (wrapped in the standard envelope so the frontend client can
// unwrap it, then offer the string as a file download).
func (h *SkillHandler) ExportSkill(c *gin.Context) {
	skillID := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	resp, err := h.agentClient.GetSkill(ctx, &pb.GetSkillRequest{SkillId: skillID})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}
	sk := resp.GetSkill()
	if sk == nil {
		c.JSON(404, gin.H{"code": 404, "error": "skill not found"})
		return
	}

	doc := skillYAML{
		ID:           sk.Id,
		Name:         sk.Name,
		Description:  sk.Description,
		Instructions: sk.Instructions,
		Tools:        sk.Tools,
		Tags:         sk.Tags,
		Status:       sk.Status,
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": fmt.Sprintf("marshal yaml: %v", err)})
		return
	}

	c.JSON(200, gin.H{
		"code": 0,
		"data": gin.H{
			"yaml":     string(out),
			"filename": fmt.Sprintf("skill-%s.yaml", safeFilename(sk.Name)),
		},
	})
}

// ImportSkill handles POST /skills/import. Body: {"yaml": "<skill yaml>"}.
// Parses the YAML and upserts the skill: when the YAML carries an ID it routes
// through UpdateSkill (preserves created_at, bumps version); otherwise it
// creates a fresh skill (server assigns the ID). Name uniqueness is enforced
// by the store's unique index on name.
func (h *SkillHandler) ImportSkill(c *gin.Context) {
	var body struct {
		YAML string `json:"yaml"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": "invalid request body"})
		return
	}
	if strings.TrimSpace(body.YAML) == "" {
		c.JSON(400, gin.H{"code": 400, "error": "yaml field is required"})
		return
	}

	var doc skillYAML
	if err := yaml.Unmarshal([]byte(body.YAML), &doc); err != nil {
		c.JSON(400, gin.H{"code": 400, "error": fmt.Sprintf("parse yaml: %v", err)})
		return
	}
	if strings.TrimSpace(doc.Name) == "" {
		c.JSON(400, gin.H{"code": 400, "error": "skill yaml must include a name"})
		return
	}

	sk := &pb.Skill{
		Id:           doc.ID,
		Name:         doc.Name,
		Description:  doc.Description,
		Instructions: doc.Instructions,
		Tools:        doc.Tools,
		Tags:         doc.Tags,
		Status:       doc.Status,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Upsert by ID when present; otherwise create (server assigns ID).
	if sk.Id != "" {
		resp, err := h.agentClient.UpdateSkill(ctx, &pb.UpdateSkillRequest{Skill: sk})
		if err != nil {
			c.JSON(500, gin.H{"code": 500, "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"code": 0, "data": gin.H{"skill": resp.Skill, "imported": true}})
		return
	}

	resp, err := h.agentClient.CreateSkill(ctx, &pb.CreateSkillRequest{Skill: sk})
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "data": gin.H{"skill": resp.Skill, "imported": true}})
}

// safeFilename collapses a skill name into a filesystem-safe slug for the
// suggested download filename.
func safeFilename(name string) string {
	if name == "" {
		return "skill"
	}
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "skill"
	}
	return out
}
