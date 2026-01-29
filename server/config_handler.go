package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"kiro2api/auth"
	"kiro2api/logger"
	"kiro2api/types"

	"github.com/gin-gonic/gin"
)

// ImportAccountInput 导入账号的输入格式
type ImportAccountInput struct {
	RefreshToken string `json:"refreshToken"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	Region       string `json:"region"`
	Provider     string `json:"provider"`
}

// ImportResult 单个账号导入结果
type ImportResult struct {
	Index   int    `json:"index"`
	Email   string `json:"email,omitempty"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ConfigStore 配置存储管理
type ConfigStore struct {
	configs  []auth.AuthConfig
	filePath string
	mutex    sync.RWMutex
}

var configStore *ConfigStore

// InitConfigStore 初始化配置存储
func InitConfigStore(filePath string) error {
	configStore = &ConfigStore{
		filePath: filePath,
		configs:  []auth.AuthConfig{},
	}
	return configStore.load()
}

// GetConfigStore 获取配置存储实例
func GetConfigStore() *ConfigStore {
	return configStore
}

// load 从文件加载配置
func (cs *ConfigStore) load() error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	data, err := os.ReadFile(cs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			cs.configs = []auth.AuthConfig{}
			return nil
		}
		return err
	}

	var configs []auth.AuthConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return err
	}

	cs.configs = configs
	return nil
}

// save 保存配置到文件
func (cs *ConfigStore) save() error {
	data, err := json.MarshalIndent(cs.configs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cs.filePath, data, 0600)
}

// GetConfigs 获取所有配置
func (cs *ConfigStore) GetConfigs() []auth.AuthConfig {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	result := make([]auth.AuthConfig, len(cs.configs))
	copy(result, cs.configs)
	return result
}

// AddConfig 添加配置
func (cs *ConfigStore) AddConfig(config auth.AuthConfig) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cs.configs = append(cs.configs, config)
	return cs.save()
}

// UpdateConfig 更新配置
func (cs *ConfigStore) UpdateConfig(index int, config auth.AuthConfig) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if index < 0 || index >= len(cs.configs) {
		return os.ErrNotExist
	}

	cs.configs[index] = config
	return cs.save()
}

// DeleteConfig 删除配置
func (cs *ConfigStore) DeleteConfig(index int) error {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if index < 0 || index >= len(cs.configs) {
		return os.ErrNotExist
	}

	cs.configs = append(cs.configs[:index], cs.configs[index+1:]...)
	return cs.save()
}

// handleGetConfig 获取配置列表
func handleGetConfig(c *gin.Context) {
	if configStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "配置存储未初始化"})
		return
	}

	configs := configStore.GetConfigs()

	// 返回配置（隐藏敏感信息的完整版本供编辑使用）
	c.JSON(http.StatusOK, gin.H{
		"configs": configs,
		"count":   len(configs),
	})
}

// handleAddConfig 添加配置
func handleAddConfig(c *gin.Context) {
	if configStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "配置存储未初始化"})
		return
	}

	var config auth.AuthConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据: " + err.Error()})
		return
	}

	// 验证必要字段
	if config.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "RefreshToken不能为空"})
		return
	}

	// 设置默认认证类型
	if config.AuthType == "" {
		config.AuthType = auth.AuthMethodSocial
	}

	// IdC认证验证
	if config.AuthType == auth.AuthMethodIdC {
		if config.ClientID == "" || config.ClientSecret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "IdC认证需要ClientID和ClientSecret"})
			return
		}
	}

	if err := configStore.AddConfig(config); err != nil {
		logger.Error("添加配置失败", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败"})
		return
	}

	logger.Info("添加Token配置成功", logger.String("auth_type", config.AuthType))
	c.JSON(http.StatusOK, gin.H{"message": "配置添加成功"})
}

// handleUpdateConfig 更新配置
func handleUpdateConfig(c *gin.Context) {
	if configStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "配置存储未初始化"})
		return
	}

	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的索引"})
		return
	}

	var config auth.AuthConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求数据: " + err.Error()})
		return
	}

	// 验证必要字段
	if config.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "RefreshToken不能为空"})
		return
	}

	if config.AuthType == "" {
		config.AuthType = auth.AuthMethodSocial
	}

	if config.AuthType == auth.AuthMethodIdC {
		if config.ClientID == "" || config.ClientSecret == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "IdC认证需要ClientID和ClientSecret"})
			return
		}
	}

	if err := configStore.UpdateConfig(index, config); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
			return
		}
		logger.Error("更新配置失败", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败"})
		return
	}

	logger.Info("更新Token配置成功", logger.Int("index", index))
	c.JSON(http.StatusOK, gin.H{"message": "配置更新成功"})
}

// handleDeleteConfig 删除配置
func handleDeleteConfig(c *gin.Context) {
	if configStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "配置存储未初始化"})
		return
	}

	indexStr := c.Param("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的索引"})
		return
	}

	if err := configStore.DeleteConfig(index); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "配置不存在"})
			return
		}
		logger.Error("删除配置失败", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除配置失败"})
		return
	}

	logger.Info("删除Token配置成功", logger.Int("index", index))
	c.JSON(http.StatusOK, gin.H{"message": "配置删除成功"})
}

// handleImportConfig 批量导入配置（自动刷新获取完整信息）
func handleImportConfig(c *gin.Context) {
	if configStore == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "配置存储未初始化"})
		return
	}

	var inputs []ImportAccountInput
	if err := c.ShouldBindJSON(&inputs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的JSON数据: " + err.Error()})
		return
	}

	if len(inputs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "导入数据为空"})
		return
	}

	results := make([]ImportResult, 0, len(inputs))
	successCount := 0

	for i, input := range inputs {
		result := ImportResult{Index: i}

		if input.RefreshToken == "" {
			result.Status = "error"
			result.Message = "RefreshToken为空"
			results = append(results, result)
			continue
		}

		// 判断认证类型：有 clientId 和 clientSecret 则为 IdC
		isIdC := input.ClientID != "" && input.ClientSecret != ""

		var authConfig auth.AuthConfig
		var tokenInfo types.TokenInfo
		var err error

		if isIdC {
			authConfig = auth.AuthConfig{
				AuthType:     auth.AuthMethodIdC,
				RefreshToken: input.RefreshToken,
				ClientID:     input.ClientID,
				ClientSecret: input.ClientSecret,
			}
			tokenInfo, err = auth.RefreshIdCToken(authConfig)
		} else {
			authConfig = auth.AuthConfig{
				AuthType:     auth.AuthMethodSocial,
				RefreshToken: input.RefreshToken,
			}
			tokenInfo, err = auth.RefreshSocialToken(input.RefreshToken)
		}

		if err != nil {
			result.Status = "error"
			result.Message = "刷新Token失败: " + err.Error()
			results = append(results, result)
			logger.Warn("导入账号刷新Token失败", logger.Int("index", i), logger.Err(err))
			continue
		}

		// 获取用量信息
		checker := auth.NewUsageLimitsChecker()
		usageResult := checker.CheckUsageLimitsWithStatus(tokenInfo)

		if usageResult.Status == types.AccountStatusBanned {
			result.Status = "banned"
			result.Message = "账号已封禁: " + usageResult.BanReason
			if usageResult.UsageLimits != nil && usageResult.UsageLimits.UserInfo.Email != "" {
				result.Email = usageResult.UsageLimits.UserInfo.Email
			}
			results = append(results, result)
			logger.Warn("导入账号已封禁", logger.Int("index", i), logger.String("reason", usageResult.BanReason))
			continue
		}

		if usageResult.Error != nil {
			result.Status = "error"
			result.Message = "获取用量失败: " + usageResult.Error.Error()
			results = append(results, result)
			logger.Warn("导入账号获取用量失败", logger.Int("index", i), logger.Err(usageResult.Error))
			continue
		}

		// 获取邮箱
		email := "unknown"
		if usageResult.UsageLimits != nil && usageResult.UsageLimits.UserInfo.Email != "" {
			email = usageResult.UsageLimits.UserInfo.Email
		}
		result.Email = email

		// 更新 authConfig 的 refreshToken（可能已更新）
		if tokenInfo.RefreshToken != "" {
			authConfig.RefreshToken = tokenInfo.RefreshToken
		}

		// 保存配置
		if err := configStore.AddConfig(authConfig); err != nil {
			result.Status = "error"
			result.Message = "保存配置失败: " + err.Error()
			results = append(results, result)
			logger.Error("导入账号保存失败", logger.Int("index", i), logger.Err(err))
			continue
		}

		result.Status = "success"
		result.Message = "导入成功"
		results = append(results, result)
		successCount++

		logger.Info("导入账号成功",
			logger.Int("index", i),
			logger.String("email", email),
			logger.String("auth_type", authConfig.AuthType),
			logger.Float64("available", usageResult.Available))

		// 避免请求过快
		if i < len(inputs)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   len(inputs),
		"success": successCount,
		"failed":  len(inputs) - successCount,
		"results": results,
	})
}
