package ai

import (
	"context"

	"agent-flow/internal/config"
)

// AIService AI 服务接口
type AIService interface {
	Chat(ctx context.Context, systemPrompt, userMessage string) (string, error)
	Check(ctx context.Context) error
	GetModel() string
}

// Service AI 服务实现
type Service struct {
	config  *config.AIConfig
	planner AIService
	worker  AIService
	monitor AIService
}

// NewService 创建 AI 服务
func NewService(cfg *config.AIConfig) *Service {
	return &Service{
		config: cfg,
	}
}

// Init 初始化所有角色服务
func (s *Service) Init() error {
	// 初始化架构服务
	var err error
	s.planner, err = s.createPlannerService()
	if err != nil {
		return err
	}

	// 初始化执行者服务
	s.worker, err = s.createWorkerService()
	if err != nil {
		return err
	}

	// 初始化监工服务
	s.monitor, err = s.createMonitorService()
	if err != nil {
		return err
	}

	return nil
}

// CreatePlannerService 创建架构 AI 服务
func (s *Service) createPlannerService() (AIService, error) {
	if s.config.IsPlannerLocal() {
		return NewOllamaClient(&s.config.Planner.Local), nil
	}
	return NewRemoteClient(&s.config.Planner.Remote), nil
}

// CreateWorkerService 创建执行者 AI 服务
func (s *Service) createWorkerService() (AIService, error) {
	if s.config.IsWorkerLocal() {
		return NewOllamaClient(&s.config.Worker.Local), nil
	}
	return NewRemoteClient(&s.config.Worker.Remote), nil
}

// CreateMonitorService 创建监工 AI 服务
func (s *Service) createMonitorService() (AIService, error) {
	if s.config.IsMonitorLocal() {
		return NewOllamaClient(&s.config.Monitor.Local), nil
	}
	return NewRemoteClient(&s.config.Monitor.Remote), nil
}

// GetPlannerService 获取架构服务
func (s *Service) GetPlannerService() AIService {
	return s.planner
}

// GetWorkerService 获取执行者服务
func (s *Service) GetWorkerService() AIService {
	return s.worker
}

// GetMonitorService 获取监工服务
func (s *Service) GetMonitorService() AIService {
	return s.monitor
}

// Config returns the loaded AI configuration.
func (s *Service) Config() *config.AIConfig {
	return s.config
}

// PlannerChat 调用架构 AI
func (s *Service) PlannerChat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	return s.planner.Chat(ctx, systemPrompt, userMessage)
}

// WorkerChat 调用执行者 AI
func (s *Service) WorkerChat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	return s.worker.Chat(ctx, systemPrompt, userMessage)
}

// MonitorChat 调用监工 AI
func (s *Service) MonitorChat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	return s.monitor.Chat(ctx, systemPrompt, userMessage)
}

// CheckAll 检查所有 AI 服务
func (s *Service) CheckAll(ctx context.Context) error {
	if err := s.planner.Check(ctx); err != nil {
		return err
	}
	if err := s.worker.Check(ctx); err != nil {
		return err
	}
	if err := s.monitor.Check(ctx); err != nil {
		return err
	}
	return nil
}
