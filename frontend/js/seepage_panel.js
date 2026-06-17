const SeepagePanel = (function() {
    let seepageChart = null;
    let pressureChart = null;
    let activeAlarms = [];
    let sensorConfigs = [];
    let latestSensorData = {};
    let lastSimulationData = null;

    const API_BASE = '/api/v1';

    async function apiGet(path) {
        const res = await fetch(API_BASE + path);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
    }

    async function apiPost(path, data) {
        const res = await fetch(API_BASE + path, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        if (!res.ok) {
            const err = await res.json().catch(() => ({}));
            throw new Error(err.error || `HTTP ${res.status}`);
        }
        return res.json();
    }

    async function apiPut(path, data) {
        const res = await fetch(API_BASE + path, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
    }

    function formatNumber(num, decimals = 2) {
        if (num == null || isNaN(num)) return '--';
        return Number(num).toFixed(decimals);
    }

    function formatDateTime(isoString) {
        if (!isoString) return '--';
        return new Date(isoString).toLocaleString('zh-CN', { hour12: false });
    }

    function getSensorValue(sensorId) {
        const data = latestSensorData[sensorId];
        return data ? data.sensor_value : null;
    }

    function showLoading(text, progress) {
        document.getElementById('loadingModal').style.display = 'flex';
        document.getElementById('loadingText').textContent = text;
        if (progress != null) {
            document.getElementById('progressFill').style.width = progress + '%';
        }
    }

    function hideLoading() {
        document.getElementById('loadingModal').style.display = 'none';
    }

    function showNotification(message, type = 'info') {
        const colors = {
            info: '#3498db',
            success: '#2ecc71',
            warning: '#f39c12',
            danger: '#e74c3c'
        };
        const notification = document.createElement('div');
        notification.style.cssText = `
            position: fixed; top: 100px; left: 50%; transform: translateX(-50%);
            background: ${colors[type]}; color: white; padding: 15px 30px;
            border-radius: 8px; z-index: 2000; box-shadow: 0 4px 20px rgba(0,0,0,0.3);
            font-weight: 600; animation: slideDown 0.3s ease;
        `;
        notification.textContent = message;
        document.body.appendChild(notification);
        setTimeout(() => notification.remove(), 3500);
    }

    function setStatus(elementId, text, active) {
        const dot = document.getElementById(elementId);
        const txt = document.getElementById(elementId + 'Text');
        if (active) {
            dot.classList.add('active');
        } else {
            dot.classList.remove('active');
        }
        if (txt) txt.textContent = text;
    }

    function updateDataCard(id, value, unit, configId) {
        const el = document.getElementById(id);
        if (!el) return;
        el.textContent = value != null ? `${formatNumber(value)} ${unit}` : `-- ${unit}`;

        const card = el.closest('.data-card');
        card.classList.remove('loading', 'warning', 'danger');

        if (value == null) {
            card.classList.add('loading');
        } else if (configId) {
            const cfg = sensorConfigs.find(s => s.sensor_id === configId);
            updateCardAlarmLevel(card, value, cfg);
        }
    }

    function updateCardAlarmLevel(card, value, cfg) {
        if (!card || !cfg || value == null) return;
        card.classList.remove('warning', 'danger');
        if (cfg.danger_threshold && value >= cfg.danger_threshold) {
            card.classList.add('danger');
        } else if (cfg.warning_threshold && value >= cfg.warning_threshold) {
            card.classList.add('warning');
        }
    }

    function updateRealTimeDataDisplay() {
        const wl001 = getSensorValue('WL-001');
        const wl002 = getSensorValue('WL-002');
        const sm001 = getSensorValue('SM-001');
        const sd001 = getSensorValue('SD-001');

        const pzSensors = ['PZ-001', 'PZ-002', 'PZ-003', 'PZ-004', 'PZ-005']
            .map(id => getSensorValue(id)).filter(v => v != null);
        const maxPZ = pzSensors.length ? Math.max(...pzSensors) : null;
        const avgPZ = pzSensors.length ? pzSensors.reduce((a, b) => a + b, 0) / pzSensors.length : null;

        updateDataCard('wl001', wl001, 'm');
        updateDataCard('wl002', wl002, 'm');
        updateDataCard('sm001', sm001, 'L/s', 'SM-001');
        updateDataCard('sd001', sd001, 'm', 'SD-001');

        const maxPZEl = document.getElementById('maxPZ');
        if (maxPZEl) {
            maxPZEl.textContent = formatNumber(maxPZ) + ' kPa';
            const card = maxPZEl.closest('.data-card');
            const cfg = sensorConfigs.find(s => s.sensor_id === 'PZ-003');
            updateCardAlarmLevel(card, maxPZ, cfg);
        }

        const avgPZEl = document.getElementById('avgPZ');
        if (avgPZEl) {
            avgPZEl.textContent = formatNumber(avgPZ) + ' kPa';
        }
    }

    function updateViewerStats() {
        const stats = document.getElementById('viewerStats');
        if (!stats) return;

        let html = '';
        const wl001 = getSensorValue('WL-001');
        const wl002 = getSensorValue('WL-002');
        const sm001 = getSensorValue('SM-001');

        if (wl001 != null) html += `<div>上游水位: <span class="stat-highlight">${formatNumber(wl001)} m</span></div>`;
        if (wl002 != null) html += `<div>下游水位: <span class="stat-highlight">${formatNumber(wl002)} m</span></div>`;
        if (sm001 != null) html += `<div>渗流量: <span class="stat-highlight">${formatNumber(sm001, 3)} L/s</span></div>`;

        if (lastSimulationData && lastSimulationData.simulation) {
            const sim = lastSimulationData.simulation;
            html += `<hr style="border-color:#2c3e50;margin:8px 0">`;
            html += `<div>仿真渗流量: <span class="stat-highlight">${formatNumber(sim.total_seepage_flow * 1000, 3)} L/s</span></div>`;
            html += `<div>最大扬压力: <span class="stat-highlight">${formatNumber(sim.max_pore_pressure, 1)} kPa</span></div>`;
            html += `<div>仿真网格: ${sim.grid_count} 个</div>`;
            html += `<div>计算耗时: ${sim.calculation_time_ms} ms</div>`;
        }

        if (!html) {
            html = '<div>等待监测数据...</div>';
        }

        stats.innerHTML = html;
    }

    function initCharts() {
        const seepageCtx = document.getElementById('seepageChart').getContext('2d');
        seepageChart = new Chart(seepageCtx, {
            type: 'line',
            data: {
                datasets: [{
                    label: '渗流量 (L/s)',
                    data: [],
                    borderColor: '#3498db',
                    backgroundColor: 'rgba(52, 152, 219, 0.1)',
                    fill: true,
                    tension: 0.4,
                    pointRadius: 2,
                    borderWidth: 2
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    x: {
                        type: 'time',
                        time: { displayFormats: { hour: 'HH:mm' } },
                        grid: { color: 'rgba(255,255,255,0.05)' },
                        ticks: { color: '#95a5a6', maxTicksLimit: 8 }
                    },
                    y: {
                        grid: { color: 'rgba(255,255,255,0.05)' },
                        ticks: { color: '#95a5a6' }
                    }
                }
            }
        });

        const pressureCtx = document.getElementById('pressureChart').getContext('2d');
        pressureChart = new Chart(pressureCtx, {
            type: 'bar',
            data: {
                labels: [],
                datasets: [{
                    label: '扬压力 (kPa)',
                    data: [],
                    backgroundColor: [],
                    borderColor: [],
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { display: false } },
                scales: {
                    x: {
                        grid: { display: false },
                        ticks: { color: '#95a5a6' }
                    },
                    y: {
                        grid: { color: 'rgba(255,255,255,0.05)' },
                        ticks: { color: '#95a5a6' }
                    }
                }
            }
        });
    }

    async function loadSensorChartData(sensorId, hours = 24) {
        try {
            const data = await apiGet(`/sensors/${sensorId}/data?hours=${hours}`);
            return data.map(d => ({
                x: new Date(d.time),
                y: d.sensor_value
            }));
        } catch (e) {
            console.error(`Failed to load ${sensorId} data:`, e);
            return [];
        }
    }

    async function refreshCharts() {
        if (seepageChart) {
            const smData = await loadSensorChartData('SM-001');
            seepageChart.data.datasets[0].data = smData;
            seepageChart.update();
        }

        if (pressureChart) {
            const pzIds = ['PZ-001', 'PZ-002', 'PZ-003', 'PZ-004', 'PZ-005'];
            const labels = [];
            const values = [];
            const bgColors = [];
            const borderColors = [];

            for (const id of pzIds) {
                const cfg = sensorConfigs.find(s => s.sensor_id === id);
                const val = getSensorValue(id);
                labels.push(id);
                values.push(val || 0);

                let color = 'rgba(46, 204, 113, 0.7)';
                let border = '#27ae60';
                if (val != null && cfg) {
                    if (cfg.danger_threshold && val >= cfg.danger_threshold) {
                        color = 'rgba(231, 76, 60, 0.7)';
                        border = '#c0392b';
                    } else if (cfg.warning_threshold && val >= cfg.warning_threshold) {
                        color = 'rgba(243, 156, 18, 0.7)';
                        border = '#d68910';
                    }
                }
                bgColors.push(color);
                borderColors.push(border);
            }

            pressureChart.data.labels = labels;
            pressureChart.data.datasets[0].data = values;
            pressureChart.data.datasets[0].backgroundColor = bgColors;
            pressureChart.data.datasets[0].borderColor = borderColors;
            pressureChart.update();
        }
    }

    async function loadSensorConfigs() {
        try {
            sensorConfigs = await apiGet('/sensors');
            return sensorConfigs;
        } catch (e) {
            console.error('Failed to load sensors:', e);
            return [];
        }
    }

    async function loadLatestSensorData() {
        try {
            const data = await apiGet('/sensors/latest');
            latestSensorData = {};
            for (const item of data) {
                latestSensorData[item.sensor_id] = item;
            }
            return latestSensorData;
        } catch (e) {
            console.error('Failed to load latest data:', e);
            return {};
        }
    }

    async function runSimulation() {
        const upstreamWL = parseFloat(document.getElementById('simUpstream').value);
        const downstreamWL = parseFloat(document.getElementById('simDownstream').value);
        const permeability = parseFloat(document.getElementById('simPermeability').value);
        const resolution = document.getElementById('simResolution').value;
        const blanketEnabled = document.getElementById('enableBlanket').checked;

        const resMap = {
            low: { x: 40, y: 20 },
            medium: { x: 60, y: 30 },
            high: { x: 100, y: 50 }
        };

        const req = {
            upstream_water_level: upstreamWL,
            downstream_water_level: downstreamWL,
            permeability_k: permeability,
            grid_resolution_x: resMap[resolution].x,
            grid_resolution_y: resMap[resolution].y,
            simulation_name: `手工仿真_${new Date().toLocaleString('zh-CN')}`
        };

        if (blanketEnabled) {
            req.blanket_length = parseFloat(document.getElementById('blanketLength').value);
            req.blanket_thickness = parseFloat(document.getElementById('blanketThickness').value);
        }

        showLoading('正在进行有限元渗流计算...', 15);

        try {
            const updateProgress = (p, t) => {
                setTimeout(() => {
                    showLoading(t, p);
                }, p * 15);
            };

            updateProgress(35, '正在构建有限元网格...');
            updateProgress(55, '正在求解渗流场方程...');
            updateProgress(75, '正在计算扬压力分布...');
            updateProgress(90, '正在后处理结果数据...');

            const result = await apiPost('/simulations/run', req);

            lastSimulationData = {
                simulation: result.simulation,
                grids: result.grids
            };

            if (typeof window.viewer3D !== 'undefined' && window.viewer3D) {
                window.viewer3D.setSimulationData(lastSimulationData);
                if (blanketEnabled) {
                    window.viewer3D.updateBlanket(req.blanket_length, req.blanket_thickness);
                }
            }

            hideLoading();
            showNotification(`渗流仿真完成！渗流量: ${formatNumber(result.simulation.total_seepage_flow * 1000, 3)} L/s`, 'success');
            loadSimulationsTable();
            updateViewerStats();

        } catch (e) {
            hideLoading();
            showNotification('仿真失败: ' + e.message, 'danger');
        }
    }

    async function runOptimization() {
        const req = {
            upstream_water_level: parseFloat(document.getElementById('simUpstream').value),
            downstream_water_level: parseFloat(document.getElementById('simDownstream').value),
            min_blanket_length: parseFloat(document.getElementById('optLenMin').value),
            max_blanket_length: parseFloat(document.getElementById('optLenMax').value),
            min_blanket_thickness: parseFloat(document.getElementById('optThickMin').value),
            max_blanket_thickness: parseFloat(document.getElementById('optThickMax').value),
            population_size: parseInt(document.getElementById('optPopSize').value),
            max_generations: parseInt(document.getElementById('optMaxGen').value),
            mutation_rate: 0.1,
            crossover_rate: 0.8,
            optimization_name: `GA优化_${new Date().toLocaleString('zh-CN')}`
        };

        const totalSteps = req.max_generations;
        let currentStep = 0;
        showLoading('遗传算法：初始化种群...', 5);

        const progressInterval = setInterval(() => {
            currentStep = Math.min(currentStep + 1, totalSteps - 1);
            const p = Math.floor((currentStep / totalSteps) * 90) + 5;
            let msg = `遗传算法：第 ${currentStep} / ${totalSteps} 代`;
            if (p > 70) msg = '遗传算法：收敛评估中...';
            showLoading(msg, p);
        }, 300);

        try {
            const result = await apiPost('/optimizations/run', req);
            clearInterval(progressInterval);
            showLoading('优化完成，正在保存结果...', 100);

            const opt = result.optimization;
            const summary = result.summary;

            const optBox = document.getElementById('optimizationResult');
            optBox.style.display = 'block';
            optBox.innerHTML = `
                <div class="result-title">✨ 优化结果 (${formatNumber(opt.flow_reduction_rate)}% 渗流削减)</div>
                <div class="result-row"><span>最优铺盖长度:</span><span>${formatNumber(opt.blanket_length)} m</span></div>
                <div class="result-row"><span>最优铺盖厚度:</span><span>${formatNumber(opt.blanket_thickness)} m</span></div>
                <div class="result-row"><span>优化前渗流量:</span><span>${formatNumber(summary.baseline_flow_lps, 3)} L/s</span></div>
                <div class="result-row"><span>优化后渗流量:</span><span style="color:#2ecc71">${formatNumber(summary.optimized_flow_lps, 3)} L/s</span></div>
                <div class="result-row"><span>迭代代数:</span><span>${opt.generation_count} 代</span></div>
                <div class="result-row"><span>优化耗时:</span><span>${opt.optimization_time_ms} ms</span></div>
                <button class="btn btn-info" style="margin-top:10px" onclick="SeepagePanel.applyOptimizedBlanket(${opt.blanket_length}, ${opt.blanket_thickness})">
                    应用到仿真
                </button>
            `;

            setTimeout(hideLoading, 500);
            showNotification('防渗优化完成！渗流削减率: ' + formatNumber(opt.flow_reduction_rate) + '%', 'success');
            loadOptimizationsTable();

        } catch (e) {
            clearInterval(progressInterval);
            hideLoading();
            showNotification('优化失败: ' + e.message, 'danger');
        }
    }

    function applyOptimizedBlanket(length, thickness) {
        document.getElementById('enableBlanket').checked = true;
        document.getElementById('blanketControls').style.display = 'block';
        document.getElementById('blanketThickControls').style.display = 'block';
        document.getElementById('blanketLength').value = length;
        document.getElementById('blanketThickness').value = thickness;

        if (typeof window.viewer3D !== 'undefined' && window.viewer3D) {
            window.viewer3D.updateBlanket(length, thickness);
        }

        showNotification(`已应用最优参数: ${formatNumber(length)}m × ${formatNumber(thickness)}m`, 'info');
    }

    async function loadSimulationsTable() {
        try {
            const sims = await apiGet('/simulations?limit=20');
            const tbody = document.getElementById('simulationsTableBody');

            if (sims.length === 0) {
                tbody.innerHTML = '<tr><td colspan="10" class="text-center">暂无仿真记录</td></tr>';
                return;
            }

            tbody.innerHTML = sims.map(s => `
                <tr>
                    <td>${s.id}</td>
                    <td>${s.simulation_name || '-'}</td>
                    <td>${formatNumber(s.upstream_water_level)} m</td>
                    <td>${formatNumber(s.downstream_water_level)} m</td>
                    <td style="color:#3498db;font-weight:bold">${formatNumber(s.total_seepage_flow * 1000, 3)}</td>
                    <td style="color:#e67e22">${formatNumber(s.max_pore_pressure, 1)}</td>
                    <td>${s.grid_count}</td>
                    <td>${s.calculation_time_ms}</td>
                    <td>${formatDateTime(s.simulation_time)}</td>
                    <td>
                        <button class="btn btn-info" onclick="SeepagePanel.loadSimulationToViewer(${s.id})">查看</button>
                    </td>
                </tr>
            `).join('');
        } catch (e) {
            console.error(e);
        }
    }

    async function loadSimulationToViewer(id) {
        try {
            showLoading('加载仿真数据...');
            const [sim, gridRes] = await Promise.all([
                apiGet(`/simulations/${id}`),
                apiGet(`/simulations/${id}/grids`)
            ]);

            lastSimulationData = {
                simulation: sim,
                grids: gridRes.grids
            };

            if (typeof window.viewer3D !== 'undefined' && window.viewer3D) {
                window.viewer3D.setSimulationData(lastSimulationData);
            }

            hideLoading();
            showNotification('仿真数据加载完成', 'success');
            if (typeof switchTab === 'function') {
                switchTab('3dview');
            }
            updateViewerStats();
        } catch (e) {
            hideLoading();
            showNotification('加载失败: ' + e.message, 'danger');
        }
    }

    async function loadOptimizationsTable() {
        try {
            const opts = await apiGet('/optimizations?limit=10');
            const tbody = document.getElementById('optimizationsTableBody');

            if (opts.length === 0) {
                tbody.innerHTML = '<tr><td colspan="10" class="text-center">暂无优化记录</td></tr>';
                return;
            }

            tbody.innerHTML = opts.map(o => `
                <tr>
                    <td>${o.id}</td>
                    <td>${o.optimization_name || '-'}</td>
                    <td style="color:#2ecc71;font-weight:bold">${formatNumber(o.blanket_length)}</td>
                    <td style="color:#2ecc71;font-weight:bold">${formatNumber(o.blanket_thickness, 2)}</td>
                    <td>${formatNumber(o.baseline_seepage_flow * 1000, 3)}</td>
                    <td style="color:#27ae60">${formatNumber(o.optimized_seepage_flow * 1000, 3)}</td>
                    <td>
                        <span class="badge ${o.flow_reduction_rate >= 20 ? 'badge-success' : 'badge-info'}">
                            ${formatNumber(o.flow_reduction_rate)}%
                        </span>
                    </td>
                    <td>${o.generation_count}</td>
                    <td>${o.optimization_time_ms}</td>
                    <td>${formatDateTime(o.created_at)}</td>
                </tr>
            `).join('');
        } catch (e) {
            console.error(e);
        }
    }

    async function loadSensorsTable() {
        try {
            const sensors = await apiGet('/sensors');
            const tbody = document.getElementById('sensorsTableBody');

            const typeMap = {
                piezometer: { label: '扬压力计', cls: 'badge-warning' },
                seepage_meter: { label: '渗流量计', cls: 'badge-info' },
                water_level: { label: '水位计', cls: 'badge-info' },
                scour_depth: { label: '冲刷深度计', cls: 'badge-danger' },
                infiltration_line: { label: '浸润线测点', cls: 'badge-success' }
            };

            tbody.innerHTML = sensors.map(s => {
                const t = typeMap[s.sensor_type] || { label: s.sensor_type, cls: 'badge-info' };
                const value = getSensorValue(s.sensor_id);
                let statusBadge = '<span class="badge badge-success">正常</span>';
                if (value != null) {
                    if (s.danger_threshold && value >= s.danger_threshold) {
                        statusBadge = '<span class="badge badge-danger">危险</span>';
                    } else if (s.warning_threshold && value >= s.warning_threshold) {
                        statusBadge = '<span class="badge badge-warning">预警</span>';
                    }
                }
                return `
                    <tr>
                        <td style="font-weight:bold;color:#3498db">${s.sensor_id}</td>
                        <td>${s.sensor_name}</td>
                        <td><span class="badge ${t.cls}">${t.label}</span></td>
                        <td>(${formatNumber(s.location_x)}, ${formatNumber(s.location_y)})</td>
                        <td>${s.warning_threshold != null ? formatNumber(s.warning_threshold) : '-'}</td>
                        <td>${s.danger_threshold != null ? formatNumber(s.danger_threshold) : '-'}</td>
                        <td>${s.unit}</td>
                        <td>${statusBadge}</td>
                    </tr>
                `;
            }).join('');
        } catch (e) {
            console.error(e);
        }
    }

    async function loadAlarms() {
        try {
            const result = await apiGet('/alarms?limit=50&unhandled=true');
            activeAlarms = result.alarms || [];
            renderAlarms();
        } catch (e) {
            console.error(e);
        }
    }

    function renderAlarms() {
        const countEl = document.getElementById('activeAlarmCount');
        const listEl = document.getElementById('alarmList');
        const panelEl = document.getElementById('alarmPanel');

        const unhandled = activeAlarms.filter(a => !a.is_handled);
        countEl.textContent = unhandled.length;

        if (unhandled.length > 0) {
            panelEl.classList.add('active');
        } else {
            setTimeout(() => panelEl.classList.remove('active'), 3000);
        }

        if (activeAlarms.length === 0) {
            listEl.innerHTML = '<div class="empty-state">暂无告警</div>';
            return;
        }

        listEl.innerHTML = activeAlarms.slice(0, 20).map(a => {
            const levelCls = a.alarm_level === 'DANGER' ? 'danger' : '';
            const handledCls = a.is_handled ? 'handled' : '';
            const levelBadge = a.alarm_level === 'DANGER'
                ? '<span class="badge badge-danger">危险</span>'
                : '<span class="badge badge-warning">预警</span>';

            return `
                <div class="alarm-item ${levelCls} ${handledCls}">
                    <div class="alarm-title">
                        <span>${levelBadge} ${a.sensor_id || '系统'}</span>
                        <span class="alarm-time">${formatDateTime(a.alarm_time)}</span>
                    </div>
                    <div class="alarm-message">${a.alarm_message}</div>
                    ${a.sensor_value != null ? `<div class="alarm-message" style="color:#f1c40f">当前值: ${formatNumber(a.sensor_value)} / 阈值: ${formatNumber(a.threshold_value)}</div>` : ''}
                    ${!a.is_handled ? `
                        <div class="alarm-actions">
                            <button class="btn btn-info" onclick="SeepagePanel.acknowledgeAlarm(${a.id})" style="padding:5px 10px;font-size:11px">确认处理</button>
                            <button class="btn" style="padding:5px 10px;font-size:11px;background:#34495e;color:white" onclick="SeepagePanel.loadSensorDetail('${a.sensor_id}')">查看详情</button>
                        </div>
                    ` : `<div style="color:#2ecc71;font-size:11px;margin-top:5px">✓ 已由 ${a.handled_by || '系统'} 处理</div>`}
                </div>
            `;
        }).join('');
    }

    async function acknowledgeAlarm(id) {
        try {
            await apiPut(`/alarms/${id}/handle`, {
                handled_by: 'Web用户',
                handle_note: '通过Web控制台确认处理'
            });
            showNotification('告警已确认', 'success');
            loadAlarms();
        } catch (e) {
            showNotification('确认失败: ' + e.message, 'danger');
        }
    }

    function loadSensorDetail(sensorId) {
        if (!sensorId) return;
        if (typeof switchTab === 'function') {
            switchTab('sensors');
        }
        showNotification(`已定位到传感器 ${sensorId}`, 'info');
    }

    function switchTab(tabId) {
        document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

        document.querySelector(`[data-tab="${tabId}"]`).classList.add('active');
        document.getElementById(`tab-${tabId}`).classList.add('active');
    }

    function updateVisualizationOptions() {
        if (typeof window.viewer3D === 'undefined' || !window.viewer3D) return;

        const opts = {
            showStreamlines: document.getElementById('showStreamlines').checked,
            showPressureCloud: document.getElementById('showPressureCloud').checked,
            showWireframe: document.getElementById('showWireframe').checked,
            showSensors: document.getElementById('showSensors').checked,
            particleCount: parseInt(document.getElementById('particleCount').value),
            particleSpeed: parseInt(document.getElementById('particleSpeed').value)
        };

        window.viewer3D.setOptions(opts);
    }

    async function healthCheck() {
        try {
            const health = await apiGet('/health');
            setStatus('dbStatus', '数据库', health.status === 'ok');
        } catch (e) {
            setStatus('dbStatus', '数据库', false);
        }
    }

    async function runDefaultSimulation() {
        try {
            const req = {
                upstream_water_level: 8.5,
                downstream_water_level: 3.2,
                permeability_k: 1e-7,
                grid_resolution_x: 60,
                grid_resolution_y: 30,
                simulation_name: '初始默认仿真'
            };
            const result = await apiPost('/simulations/run', req);
            lastSimulationData = {
                simulation: result.simulation,
                grids: result.grids
            };
            if (typeof window.viewer3D !== 'undefined' && window.viewer3D) {
                window.viewer3D.setSimulationData(lastSimulationData);
            }
            updateViewerStats();
        } catch (e) {
            console.log('Default simulation skipped:', e.message);
        }
    }

    function setupEventListeners() {
        document.querySelectorAll('.tab-btn').forEach(btn => {
            btn.addEventListener('click', () => switchTab(btn.dataset.tab));
        });

        document.getElementById('runSimulation').addEventListener('click', runSimulation);
        document.getElementById('runOptimization').addEventListener('click', runOptimization);

        document.getElementById('enableBlanket').addEventListener('change', (e) => {
            const show = e.target.checked;
            document.getElementById('blanketControls').style.display = show ? 'block' : 'none';
            document.getElementById('blanketThickControls').style.display = show ? 'block' : 'none';
        });

        ['showStreamlines', 'showPressureCloud', 'showWireframe', 'showSensors'].forEach(id => {
            document.getElementById(id).addEventListener('change', updateVisualizationOptions);
        });

        document.getElementById('particleCount').addEventListener('input', updateVisualizationOptions);
        document.getElementById('particleSpeed').addEventListener('input', updateVisualizationOptions);
    }

    function getState() {
        return {
            sensorConfigs,
            latestSensorData,
            lastSimulationData,
            activeAlarms
        };
    }

    function getViewer3DUpdateData() {
        const dataMap = {};
        for (const [id, data] of Object.entries(latestSensorData)) {
            dataMap[id] = data.sensor_value;
        }
        return {
            sensorConfigs,
            dataMap
        };
    }

    return {
        initCharts,
        setupEventListeners,
        loadSensorConfigs,
        loadLatestSensorData,
        updateRealTimeDataDisplay,
        updateViewerStats,
        refreshCharts,
        runSimulation,
        runOptimization,
        applyOptimizedBlanket,
        loadSimulationsTable,
        loadSimulationToViewer,
        loadOptimizationsTable,
        loadSensorsTable,
        loadAlarms,
        renderAlarms,
        acknowledgeAlarm,
        loadSensorDetail,
        switchTab,
        healthCheck,
        runDefaultSimulation,
        updateVisualizationOptions,
        getSensorValue,
        getState,
        getViewer3DUpdateData,
        showNotification,
        showLoading,
        hideLoading
    };
})();
