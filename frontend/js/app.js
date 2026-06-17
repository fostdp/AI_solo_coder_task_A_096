let viewer3D = null;

function updateCurrentTime() {
    const now = new Date();
    document.getElementById('currentTime').textContent =
        now.toLocaleString('zh-CN', { hour12: false });
}

async function main() {
    try {
        const style = document.createElement('style');
        style.textContent = `
            @keyframes slideDown {
                from { transform: translate(-50%, -50px); opacity: 0; }
                to { transform: translate(-50%, 0); opacity: 1; }
            }
        `;
        document.head.appendChild(style);

        updateCurrentTime();
        setInterval(updateCurrentTime, 1000);

        SeepagePanel.setupEventListeners();
        SeepagePanel.initCharts();

        viewer3D = new TuoshanDam3D('threeCanvasContainer');
        window.viewer3D = viewer3D;

        SeepagePanel.healthCheck();
        setInterval(SeepagePanel.healthCheck, 30000);

        await SeepagePanel.loadSensorConfigs();
        await SeepagePanel.loadLatestSensorData();
        SeepagePanel.updateRealTimeDataDisplay();
        SeepagePanel.updateViewerStats();

        if (viewer3D) {
            const viewerData = SeepagePanel.getViewer3DUpdateData();
            viewer3D.updateSensorConfigs(viewerData.sensorConfigs, viewerData.dataMap);
        }

        SeepagePanel.loadSimulationsTable();
        SeepagePanel.loadOptimizationsTable();
        SeepagePanel.loadSensorsTable();
        SeepagePanel.loadAlarms();

        SeepagePanel.refreshCharts();

        setInterval(async () => {
            await SeepagePanel.loadLatestSensorData();
            SeepagePanel.updateRealTimeDataDisplay();
            SeepagePanel.updateViewerStats();

            if (viewer3D) {
                const viewerData = SeepagePanel.getViewer3DUpdateData();
                viewer3D.updateSensorConfigs(viewerData.sensorConfigs, viewerData.dataMap);
            }
        }, 30000);

        setInterval(SeepagePanel.loadAlarms, 15000);
        setInterval(SeepagePanel.refreshCharts, 60000);
        setInterval(SeepagePanel.loadSensorsTable, 30000);
        setInterval(SeepagePanel.loadSimulationsTable, 60000);

        setTimeout(SeepagePanel.runDefaultSimulation, 2000);

        console.log('%c🏯 它山堰渗流仿真系统', 'font-size:20px;color:#3498db;font-weight:bold');
        console.log('%c系统初始化完成', 'font-size:12px;color:#2ecc71');

    } catch (e) {
        console.error('初始化失败:', e);
        SeepagePanel.showNotification('系统初始化出错: ' + e.message, 'danger');
    }
}

document.addEventListener('DOMContentLoaded', main);
