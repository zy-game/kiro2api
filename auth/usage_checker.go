package auth

import (
	"fmt"
	"io"
	"kiro2api/logger"
	"kiro2api/types"
	"kiro2api/utils"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// UsageCheckResult 用量检查结果
type UsageCheckResult struct {
	UsageLimits *types.UsageLimits
	Status      string  // active, exhausted, banned, error
	BanReason   string  // 封禁原因（如果被封禁）
	Available   float64 // 可用次数
	TotalLimit  float64 // 总配额
	TotalUsed   float64 // 已使用
	Error       error   // 错误信息
}

// UsageLimitsChecker 使用限制检查器 (遵循SRP原则)
type UsageLimitsChecker struct {
	httpClient *http.Client
}

// NewUsageLimitsChecker 创建使用限制检查器
func NewUsageLimitsChecker() *UsageLimitsChecker {
	return &UsageLimitsChecker{
		httpClient: utils.SharedHTTPClient,
	}
}

// CheckUsageLimitsWithStatus 检查token的使用限制并返回详细状态
func (c *UsageLimitsChecker) CheckUsageLimitsWithStatus(token types.TokenInfo) *UsageCheckResult {
	result := &UsageCheckResult{
		Status: types.AccountStatusError,
	}

	// 构建请求URL
	baseURL := "https://codewhisperer.us-east-1.amazonaws.com/getUsageLimits"
	params := url.Values{}
	params.Add("isEmailRequired", "true")
	params.Add("origin", "AI_EDITOR")
	params.Add("resourceType", "AGENTIC_REQUEST")

	requestURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// 创建HTTP请求
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		result.Error = fmt.Errorf("创建请求失败: %v", err)
		return result
	}

	// 设置请求头
	req.Header.Set("x-amz-user-agent", "aws-sdk-js/1.0.0 KiroIDE-0.6.18-66c23a8c5d15afabec89ef9954ef52a119f10d369df04d548fc6c1eac694b0d1")
	req.Header.Set("user-agent", "aws-sdk-js/1.0.0 ua/2.1 os/windows lang/js md/nodejs#20.16.0 api/codewhispererruntime#1.0.0 m/E KiroIDE-0.6.18-66c23a8c5d15afabec89ef9954ef52a119f10d369df04d548fc6c1eac694b0d1")
	req.Header.Set("host", "codewhisperer.us-east-1.amazonaws.com")
	req.Header.Set("amz-sdk-invocation-id", generateInvocationID())
	req.Header.Set("amz-sdk-request", "attempt=1; max=1")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Set("Connection", "close")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		result.Error = fmt.Errorf("请求失败: %v", err)
		return result
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("读取响应失败: %v", err)
		return result
	}

	// 处理非200响应
	if resp.StatusCode != http.StatusOK {
		// 尝试解析错误响应，检查是否被封禁
		var errorResp map[string]any
		if err := utils.SafeUnmarshal(body, &errorResp); err == nil {
			if reason, ok := errorResp["reason"].(string); ok {
				result.Status = types.AccountStatusBanned
				result.BanReason = reason
				result.Error = fmt.Errorf("账号被封禁: %s", reason)
				logger.Warn("账号被封禁",
					logger.String("reason", reason),
					logger.Int("status_code", resp.StatusCode))
				return result
			}
		}
		result.Error = fmt.Errorf("请求失败: 状态码 %d, 响应: %s", resp.StatusCode, string(body))
		return result
	}

	// 解析响应
	var usageLimits types.UsageLimits
	if err := utils.SafeUnmarshal(body, &usageLimits); err != nil {
		result.Error = fmt.Errorf("解析响应失败: %v", err)
		return result
	}

	result.UsageLimits = &usageLimits

	// 计算用量
	for _, breakdown := range usageLimits.UsageBreakdownList {
		if breakdown.ResourceType == "CREDIT" {
			// 基础额度
			result.TotalLimit = breakdown.UsageLimitWithPrecision
			result.TotalUsed = breakdown.CurrentUsageWithPrecision

			// 免费试用额度
			if breakdown.FreeTrialInfo != nil && breakdown.FreeTrialInfo.FreeTrialStatus == "ACTIVE" {
				result.TotalLimit += breakdown.FreeTrialInfo.UsageLimitWithPrecision
				result.TotalUsed += breakdown.FreeTrialInfo.CurrentUsageWithPrecision
			}

			// 奖励额度
			for _, bonus := range breakdown.Bonuses {
				result.TotalLimit += bonus.UsageLimit
				result.TotalUsed += bonus.CurrentUsage
			}

			result.Available = result.TotalLimit - result.TotalUsed
			if result.Available < 0 {
				result.Available = 0
			}
			break
		}
	}

	// 确定状态
	if result.Available > 0 {
		result.Status = types.AccountStatusActive
	} else {
		result.Status = types.AccountStatusExhausted
	}

	// 记录日志
	logger.Debug("用量检查完成",
		logger.String("status", result.Status),
		logger.Float64("available", result.Available),
		logger.Float64("total_limit", result.TotalLimit),
		logger.Float64("total_used", result.TotalUsed),
		logger.String("user_email", usageLimits.UserInfo.Email))

	return result
}

// CheckUsageLimits 检查token的使用限制 (保持向后兼容)
func (c *UsageLimitsChecker) CheckUsageLimits(token types.TokenInfo) (*types.UsageLimits, error) {
	result := c.CheckUsageLimitsWithStatus(token)
	if result.Error != nil {
		// 如果是封禁错误，返回特殊格式便于上层识别
		if result.Status == types.AccountStatusBanned {
			return nil, fmt.Errorf("BANNED:%s", result.BanReason)
		}
		return nil, result.Error
	}
	return result.UsageLimits, nil
}

// IsBannedError 检查错误是否为封禁错误
func IsBannedError(err error) (bool, string) {
	if err == nil {
		return false, ""
	}
	errStr := err.Error()
	if strings.HasPrefix(errStr, "BANNED:") {
		return true, strings.TrimPrefix(errStr, "BANNED:")
	}
	return false, ""
}

// logUsageLimits 记录使用限制的关键信息
func (c *UsageLimitsChecker) logUsageLimits(limits *types.UsageLimits) {
	for _, breakdown := range limits.UsageBreakdownList {
		if breakdown.ResourceType == "CREDIT" {
			// 计算可用次数 (使用浮点精度数据)
			var totalLimit float64
			var totalUsed float64

			// 基础额度
			baseLimit := breakdown.UsageLimitWithPrecision
			baseUsed := breakdown.CurrentUsageWithPrecision
			totalLimit += baseLimit
			totalUsed += baseUsed

			// 免费试用额度
			var freeTrialLimit float64
			var freeTrialUsed float64
			if breakdown.FreeTrialInfo != nil && breakdown.FreeTrialInfo.FreeTrialStatus == "ACTIVE" {
				freeTrialLimit = breakdown.FreeTrialInfo.UsageLimitWithPrecision
				freeTrialUsed = breakdown.FreeTrialInfo.CurrentUsageWithPrecision
				totalLimit += freeTrialLimit
				totalUsed += freeTrialUsed
			}

			available := totalLimit - totalUsed

			logger.Info("CREDIT使用状态",
				logger.String("resource_type", breakdown.ResourceType),
				logger.Float64("total_limit", totalLimit),
				logger.Float64("total_used", totalUsed),
				logger.Float64("available", available),
				logger.Float64("base_limit", baseLimit),
				logger.Float64("base_used", baseUsed),
				logger.Float64("free_trial_limit", freeTrialLimit),
				logger.Float64("free_trial_used", freeTrialUsed),
				logger.String("free_trial_status", func() string {
					if breakdown.FreeTrialInfo != nil {
						return breakdown.FreeTrialInfo.FreeTrialStatus
					}
					return "NONE"
				}()))

			if available <= 1 {
				logger.Warn("CREDIT使用量即将耗尽",
					logger.Float64("remaining", available),
					logger.String("recommendation", "考虑切换到其他token"))
			}

			break
		}
	}

	// 记录订阅信息
	logger.Debug("订阅信息",
		logger.String("subscription_type", limits.SubscriptionInfo.Type),
		logger.String("subscription_title", limits.SubscriptionInfo.SubscriptionTitle),
		logger.String("user_email", limits.UserInfo.Email))
}

// generateInvocationID 生成请求ID (简化版本)
func generateInvocationID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), "kiro2api")
}
