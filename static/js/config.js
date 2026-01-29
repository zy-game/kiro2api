/**
 * Token配置管理 - 前端控制器
 */

class ConfigManager {
    constructor() {
        this.configs = [];
        this.deleteIndex = -1;
        this.apiBaseUrl = '/api';
        this.init();
    }

    init() {
        this.loadConfigs();
    }

    async loadConfigs() {
        const tbody = document.getElementById('configTableBody');
        this.showLoading(tbody);

        try {
            const response = await fetch(`${this.apiBaseUrl}/config`);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const data = await response.json();
            this.configs = data.configs || [];
            this.renderTable();
        } catch (error) {
            console.error('加载配置失败:', error);
            this.showError(tbody, `加载失败: ${error.message}`);
        }
    }

    renderTable() {
        const tbody = document.getElementById('configTableBody');

        if (this.configs.length === 0) {
            tbody.innerHTML = `
                <tr>
                    <td colspan="6" class="loading">
                        暂无配置，点击"添加Token配置"开始
                    </td>
                </tr>
            `;
            return;
        }

        const rows = this.configs.map((config, index) => this.createRow(config, index)).join('');
        tbody.innerHTML = rows;
    }

    createRow(config, index) {
        const statusClass = config.disabled ? 'status-disabled' : 'status-enabled';
        const statusText = config.disabled ? '已禁用' : '启用';
        const tokenPreview = this.maskToken(config.refreshToken);
        const clientIdPreview = config.clientId ? this.maskToken(config.clientId) : '-';

        return `
            <tr>
                <td>${index + 1}</td>
                <td>${config.auth || 'Social'}</td>
                <td><span class="token-mask">${tokenPreview}</span></td>
                <td><span class="token-mask">${clientIdPreview}</span></td>
                <td><span class="${statusClass}">${statusText}</span></td>
                <td>
                    <button class="action-btn edit-btn" onclick="configManager.showEditModal(${index})">编辑</button>
                    <button class="action-btn delete-btn" onclick="configManager.showDeleteModal(${index})">删除</button>
                </td>
            </tr>
        `;
    }

    maskToken(token) {
        if (!token) return '-';
        if (token.length <= 10) return '****';
        return token.substring(0, 6) + '...' + token.substring(token.length - 4);
    }

    showAddModal() {
        document.getElementById('modalTitle').textContent = '添加Token配置';
        document.getElementById('configIndex').value = '-1';
        document.getElementById('configForm').reset();
        document.getElementById('idcFields').style.display = 'none';
        this.showModal('configModal');
    }

    showEditModal(index) {
        const config = this.configs[index];
        if (!config) return;

        document.getElementById('modalTitle').textContent = '编辑Token配置';
        document.getElementById('configIndex').value = index;
        document.getElementById('authType').value = config.auth || 'Social';
        document.getElementById('refreshToken').value = config.refreshToken || '';
        document.getElementById('clientId').value = config.clientId || '';
        document.getElementById('clientSecret').value = config.clientSecret || '';
        document.getElementById('disabled').checked = config.disabled || false;

        this.toggleIdCFields();
        this.showModal('configModal');
    }

    toggleIdCFields() {
        const authType = document.getElementById('authType').value;
        const idcFields = document.getElementById('idcFields');
        idcFields.style.display = authType === 'IdC' ? 'block' : 'none';
    }

    async saveConfig(event) {
        event.preventDefault();

        const index = parseInt(document.getElementById('configIndex').value);
        const config = {
            auth: document.getElementById('authType').value,
            refreshToken: document.getElementById('refreshToken').value.trim(),
            disabled: document.getElementById('disabled').checked
        };

        if (config.auth === 'IdC') {
            config.clientId = document.getElementById('clientId').value.trim();
            config.clientSecret = document.getElementById('clientSecret').value.trim();

            if (!config.clientId || !config.clientSecret) {
                alert('IdC认证需要填写ClientID和ClientSecret');
                return false;
            }
        }

        if (!config.refreshToken) {
            alert('RefreshToken不能为空');
            return false;
        }

        try {
            let response;
            if (index === -1) {
                response = await fetch(`${this.apiBaseUrl}/config`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(config)
                });
            } else {
                response = await fetch(`${this.apiBaseUrl}/config/${index}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(config)
                });
            }

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || '保存失败');
            }

            this.hideModal();
            await this.loadConfigs();
        } catch (error) {
            console.error('保存配置失败:', error);
            alert(`保存失败: ${error.message}`);
        }

        return false;
    }

    showDeleteModal(index) {
        this.deleteIndex = index;
        this.showModal('deleteModal');
    }

    hideDeleteModal() {
        this.deleteIndex = -1;
        this.hideModalById('deleteModal');
    }

    async confirmDelete() {
        if (this.deleteIndex === -1) return;

        try {
            const response = await fetch(`${this.apiBaseUrl}/config/${this.deleteIndex}`, {
                method: 'DELETE'
            });

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.error || '删除失败');
            }

            this.hideDeleteModal();
            await this.loadConfigs();
        } catch (error) {
            console.error('删除配置失败:', error);
            alert(`删除失败: ${error.message}`);
        }
    }

    showModal(modalId) {
        document.getElementById(modalId).classList.add('show');
    }

    hideModal() {
        this.hideModalById('configModal');
    }

    hideModalById(modalId) {
        document.getElementById(modalId).classList.remove('show');
    }

    showLoading(container) {
        container.innerHTML = `
            <tr>
                <td colspan="6" class="loading">
                    <div class="spinner"></div>
                    正在加载配置...
                </td>
            </tr>
        `;
    }

    showError(container, message) {
        container.innerHTML = `
            <tr>
                <td colspan="6" class="error">
                    ${message}
                </td>
            </tr>
        `;
    }

    // 导入功能
    showImportModal() {
        document.getElementById('importJson').value = '';
        document.getElementById('importResults').style.display = 'none';
        document.getElementById('importResults').innerHTML = '';
        document.getElementById('importBtn').disabled = false;
        document.getElementById('importBtn').textContent = '开始导入';
        this.showModal('importModal');
    }

    hideImportModal() {
        this.hideModalById('importModal');
    }

    async importConfigs() {
        const jsonInput = document.getElementById('importJson').value.trim();
        const resultsDiv = document.getElementById('importResults');
        const importBtn = document.getElementById('importBtn');

        if (!jsonInput) {
            alert('请输入JSON配置');
            return;
        }

        let configs;
        try {
            configs = JSON.parse(jsonInput);
            if (!Array.isArray(configs)) {
                throw new Error('JSON必须是数组格式');
            }
        } catch (e) {
            alert('JSON格式错误: ' + e.message);
            return;
        }

        if (configs.length === 0) {
            alert('配置数组为空');
            return;
        }

        // 禁用按钮，显示进度
        importBtn.disabled = true;
        importBtn.textContent = '导入中...';
        resultsDiv.style.display = 'block';
        resultsDiv.innerHTML = '<div class="import-progress">正在导入 ' + configs.length + ' 个配置，请稍候...</div>';

        try {
            const response = await fetch(`${this.apiBaseUrl}/config/import`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: jsonInput
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || '导入失败');
            }

            // 显示结果
            this.renderImportResults(data, resultsDiv);

            // 刷新配置列表
            await this.loadConfigs();

        } catch (error) {
            console.error('导入失败:', error);
            resultsDiv.innerHTML = `<div class="import-error">导入失败: ${error.message}</div>`;
        } finally {
            importBtn.disabled = false;
            importBtn.textContent = '开始导入';
        }
    }

    renderImportResults(data, container) {
        let html = `
            <div class="import-summary">
                <span class="summary-total">总计: ${data.total}</span>
                <span class="summary-success">成功: ${data.success}</span>
                <span class="summary-failed">失败: ${data.failed}</span>
            </div>
            <div class="import-details">
        `;

        for (const result of data.results) {
            const statusClass = result.status === 'success' ? 'result-success' : 
                               result.status === 'banned' ? 'result-banned' : 'result-error';
            const statusIcon = result.status === 'success' ? '✓' : 
                              result.status === 'banned' ? '⚠' : '✗';
            
            html += `
                <div class="import-result-item ${statusClass}">
                    <span class="result-icon">${statusIcon}</span>
                    <span class="result-index">#${result.index + 1}</span>
                    <span class="result-email">${result.email || '-'}</span>
                    <span class="result-message">${result.message}</span>
                </div>
            `;
        }

        html += '</div>';
        container.innerHTML = html;
    }
}

const configManager = new ConfigManager();
