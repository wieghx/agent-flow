package architecture

import (
	"context"
	"fmt"
	"os"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/cache"
	"agent-flow/internal/mcp"
	wfengine "agent-flow/internal/workflow"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (p *TaskPlannerEino) isMCPTask(task *agentflowiov1alpha1.Task) bool {
	return task != nil && task.Annotations != nil && task.Annotations["agentflow.io/mcp-mode"] == "true"
}

func (p *TaskPlannerEino) handleMCPTask(ctx context.Context, task *agentflowiov1alpha1.Task, sandboxSkipped bool) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	instruction := p.buildWorkerInstruction(task)

	var output string
	var err error
	if sandboxSkipped {
		output, err = p.runMCPLocalAgent(ctx, instruction)
	} else {
		output, err = p.readMCPSandboxOutput(task)
	}
	if err != nil {
		task.Status.Phase = agentflowiov1alpha1.TaskPhaseFailed
		task.Status.Message = fmt.Sprintf("MCP 任务失败: %v", err)
		now := metav1.Now()
		if task.Status.CompletionTime == nil {
			task.Status.CompletionTime = &now
			task.Status.CompletionTimeUnix = now.Unix()
		}
		if uerr := p.Status().Update(ctx, task); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{}, nil
	}

	output = strings.TrimSpace(output)
	if output == "" {
		task.Status.Phase = agentflowiov1alpha1.TaskPhaseFailed
		task.Status.Message = "MCP 任务产出为空"
		now := metav1.Now()
		if task.Status.CompletionTime == nil {
			task.Status.CompletionTime = &now
			task.Status.CompletionTimeUnix = now.Unix()
		}
		_ = p.Status().Update(ctx, task)
		return ctrl.Result{}, nil
	}

	now := metav1.Now()
	if task.Status.Output == nil {
		task.Status.Output = &agentflowiov1alpha1.TaskOutput{}
	}
	task.Status.Output.Content = output
	task.Status.Output.Format = "text"
	task.Status.Output.GeneratedAt = &now
	task.Status.Phase = agentflowiov1alpha1.TaskPhaseSucceeded
	task.Status.Message = "MCP 任务执行成功"
	if task.Status.CompletionTime == nil {
		task.Status.CompletionTime = &now
		task.Status.CompletionTimeUnix = now.Unix()
	}

	if rel := task.Annotations["agentflow.io/workflow-output"]; rel != "" && task.Labels != nil {
		if wfName := task.Labels["agentflow.io/workflow"]; wfName != "" {
			wf := &agentflowiov1alpha1.Workflow{}
			if gerr := p.Get(ctx, clientNamespacedName(task.Namespace, wfName), wf); gerr == nil {
				_ = wfengine.WriteArtifact(wf, rel, output)
			}
		}
	}

	if err := p.Status().Update(ctx, task); err != nil {
		return ctrl.Result{}, fmt.Errorf("更新 MCP Task 状态失败：%w", err)
	}
	p.publishTaskEvent(ctx, task, cache.EventStepSucceeded, task.Status.Message, 0, 0)
	p.sendTaskFeedback(task, output)
	logger.Info("MCP task completed", "task", task.Name, "outputBytes", len(output))
	return ctrl.Result{}, nil
}

func clientNamespacedName(namespace, name string) types.NamespacedName {
	return types.NamespacedName{Namespace: namespace, Name: name}
}

func (p *TaskPlannerEino) runMCPLocalAgent(ctx context.Context, instruction string) (string, error) {
	if p.AIService == nil {
		return "", fmt.Errorf("AI 服务未初始化")
	}
	agent := mcp.NewLocalAgent(mcp.LocalAgentConfig{
		Chat: func(ctx context.Context, systemPrompt, userMessage string) (string, error) {
			result, err := p.AIService.WorkerChat(ctx, systemPrompt, userMessage)
			if err != nil {
				return "", err
			}
			return result.Content, nil
		},
		Executor: mcp.NewToolExecutor(mcp.DefaultTools()),
		MaxSteps: 15,
	})
	catalog := mcp.FormatToolCatalog(mcp.DefaultTools())
	result, err := agent.Run(ctx, instruction, catalog)
	if err != nil && result == nil {
		return "", err
	}
	if result != nil && strings.TrimSpace(result.Output) != "" {
		return result.Output, nil
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("MCP agent returned empty output")
}

func (p *TaskPlannerEino) readMCPSandboxOutput(task *agentflowiov1alpha1.Task) (string, error) {
	candidates := []string{
		fmt.Sprintf("/data/outputs/%s.txt", task.Name),
		fmt.Sprintf("/data/outputs/%s/%s.txt", task.Namespace, task.Name),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("MCP sandbox output not found for %s", task.Name)
}
