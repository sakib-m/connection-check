/**
 * SpeedTest Pro - Client-side network diagnostic engine
 * Features: Silky-smooth sliding window throughput, EMA filtering, 60fps gauge lerp, Light/Dark mode
 */

document.addEventListener('DOMContentLoaded', () => {
    // DOM Elements
    const ipDisplay = document.getElementById('ip-display');
    const ispDisplay = document.getElementById('isp-display');
    const locationDisplay = document.getElementById('location-display');
    const statusBadge = document.getElementById('status-badge');
    const quotaRemaining = document.getElementById('quota-remaining');
    const startBtn = document.getElementById('start-btn');
    const restartBtn = document.getElementById('restart-btn');
    const resultModal = document.getElementById('result-modal');
    const errorAlert = document.getElementById('error-alert');
    const errorMessage = document.getElementById('error-message');
    const themeToggleBtn = document.getElementById('theme-toggle-btn');

    const metricPing = document.getElementById('metric-ping');
    const metricJitter = document.getElementById('metric-jitter');
    const metricDownload = document.getElementById('metric-download');
    const metricUpload = document.getElementById('metric-upload');

    const barPing = document.getElementById('bar-ping');
    const barJitter = document.getElementById('bar-jitter');
    const barDownload = document.getElementById('bar-download');
    const barUpload = document.getElementById('bar-upload');

    const gaugeLabel = document.getElementById('gauge-label');
    const gaugeValue = document.getElementById('gauge-value');
    const gaugeUnit = document.getElementById('gauge-unit');
    const phaseText = document.getElementById('phase-text');
    const connectionRating = document.getElementById('connection-rating');

    // Canvas & Chart references
    const gaugeCanvas = document.getElementById('gaugeCanvas');
    const gaugeCtx = gaugeCanvas ? gaugeCanvas.getContext('2d') : null;
    let throughputChart = null;

    // State Variables
    let currentIPInfo = null;
    let isTesting = false;
    let currentGaugeSpeed = 0;
    let targetGaugeSpeed = 0;
    let gaugeAnimationId = null;

    // Smoothing Variables (Exponential Moving Average & Sliding Windows)
    let smoothedDownloadSpeed = 0;
    let smoothedUploadSpeed = 0;

    // Results Store
    let finalPing = 0;
    let finalJitter = 0;
    let finalDownload = 0;
    let finalUpload = 0;

    // -------------------------------------------------------------
    // 0. Light / Dark Mode Toggle
    // -------------------------------------------------------------
    function isDarkMode() {
        return document.documentElement.classList.contains('dark');
    }

    if (themeToggleBtn) {
        themeToggleBtn.addEventListener('click', () => {
            const isDark = document.documentElement.classList.toggle('dark');
            localStorage.setItem('theme', isDark ? 'dark' : 'light');
            updateChartTheme();
            drawGauge(currentGaugeSpeed);
            if (window.lucide) lucide.createIcons();
        });
    }

    // -------------------------------------------------------------
    // 1. Throughput Chart Setup (Chart.js)
    // -------------------------------------------------------------
    function initChart() {
        const ctx = document.getElementById('throughputChart').getContext('2d');
        const downGradient = ctx.createLinearGradient(0, 0, 0, 160);
        downGradient.addColorStop(0, 'rgba(16, 185, 129, 0.35)');
        downGradient.addColorStop(1, 'rgba(16, 185, 129, 0.0)');

        const upGradient = ctx.createLinearGradient(0, 0, 0, 160);
        upGradient.addColorStop(0, 'rgba(168, 85, 247, 0.35)');
        upGradient.addColorStop(1, 'rgba(168, 85, 247, 0.0)');

        throughputChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [
                    {
                        label: 'Download (Mbps)',
                        data: [],
                        borderColor: '#10b981',
                        backgroundColor: downGradient,
                        fill: true,
                        tension: 0.4, // Silky bezier curve
                        borderWidth: 2.5,
                        pointRadius: 0,
                    },
                    {
                        label: 'Upload (Mbps)',
                        data: [],
                        borderColor: '#a855f7',
                        backgroundColor: upGradient,
                        fill: true,
                        tension: 0.4,
                        borderWidth: 2.5,
                        pointRadius: 0,
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: false,
                plugins: {
                    legend: { display: false },
                    tooltip: { enabled: true, mode: 'index', intersect: false }
                },
                scales: {
                    x: {
                        display: false,
                        grid: { display: false }
                    },
                    y: {
                        beginAtZero: true,
                        grid: { color: isDarkMode() ? 'rgba(51, 65, 85, 0.25)' : 'rgba(203, 213, 225, 0.4)' },
                        ticks: {
                            color: isDarkMode() ? '#64748b' : '#94a3b8',
                            font: { size: 10 }
                        }
                    }
                }
            }
        });
    }

    function updateChartTheme() {
        if (!throughputChart) return;
        throughputChart.options.scales.y.grid.color = isDarkMode() ? 'rgba(51, 65, 85, 0.25)' : 'rgba(203, 213, 225, 0.4)';
        throughputChart.options.scales.y.ticks.color = isDarkMode() ? '#64748b' : '#94a3b8';
        throughputChart.update('none');
    }

    // -------------------------------------------------------------
    // 2. Speedometer Gauge Rendering & 60fps Smooth Lerp
    // -------------------------------------------------------------
    function drawGauge(speed, maxSpeed = 1000) {
        if (!gaugeCtx) return;
        const width = gaugeCanvas.width;
        const height = gaugeCanvas.height;
        const cx = width / 2;
        const cy = height / 2;
        const radius = width * 0.42;
        const dark = isDarkMode();

        gaugeCtx.clearRect(0, 0, width, height);

        const startAngle = 0.75 * Math.PI; // 135 deg
        const endAngle = 2.25 * Math.PI;   // 405 deg (270 deg sweep)

        // Background Track
        gaugeCtx.beginPath();
        gaugeCtx.arc(cx, cy, radius, startAngle, endAngle);
        gaugeCtx.lineWidth = 14;
        gaugeCtx.strokeStyle = dark ? 'rgba(30, 41, 59, 0.85)' : 'rgba(226, 232, 240, 0.9)';
        gaugeCtx.lineCap = 'round';
        gaugeCtx.stroke();

        // Speed calculation with non-linear scale for fiber & broadband accuracy
        const normalized = Math.min(Math.max(speed, 0), maxSpeed);
        const percent = Math.min(Math.sqrt(normalized) / Math.sqrt(maxSpeed), 1);
        const currentAngle = startAngle + (endAngle - startAngle) * percent;

        // Active Glowing Arc
        if (percent > 0.005) {
            gaugeCtx.save();
            const grad = gaugeCtx.createLinearGradient(0, height, width, 0);
            grad.addColorStop(0, '#06b6d4');
            grad.addColorStop(0.5, '#3b82f6');
            grad.addColorStop(1, '#10b981');

            gaugeCtx.beginPath();
            gaugeCtx.arc(cx, cy, radius, startAngle, currentAngle);
            gaugeCtx.lineWidth = 14;
            gaugeCtx.strokeStyle = grad;
            gaugeCtx.lineCap = 'round';
            gaugeCtx.shadowColor = dark ? '#06b6d4' : 'rgba(6, 182, 212, 0.4)';
            gaugeCtx.shadowBlur = dark ? 12 : 6;
            gaugeCtx.stroke();
            gaugeCtx.restore();
        }

        // Tick marks
        for (let i = 0; i <= 10; i++) {
            const angle = startAngle + (endAngle - startAngle) * (i / 10);
            const innerR = radius - 16;
            const outerR = radius - 10;
            const x1 = cx + innerR * Math.cos(angle);
            const y1 = cy + innerR * Math.sin(angle);
            const x2 = cx + outerR * Math.cos(angle);
            const y2 = cy + outerR * Math.sin(angle);

            gaugeCtx.beginPath();
            gaugeCtx.moveTo(x1, y1);
            gaugeCtx.lineTo(x2, y2);
            gaugeCtx.lineWidth = 2;
            gaugeCtx.strokeStyle = dark ? 'rgba(71, 85, 105, 0.6)' : 'rgba(148, 163, 184, 0.7)';
            gaugeCtx.stroke();
        }
    }

    // 60FPS Fluid Lerp Damping
    function animateGauge() {
        const diff = targetGaugeSpeed - currentGaugeSpeed;
        if (Math.abs(diff) > 0.05) {
            currentGaugeSpeed += diff * 0.08; // Gentle fluid spring
        } else {
            currentGaugeSpeed = targetGaugeSpeed;
        }

        drawGauge(currentGaugeSpeed);
        gaugeValue.textContent = currentGaugeSpeed.toFixed(1);
        gaugeAnimationId = requestAnimationFrame(animateGauge);
    }

    // -------------------------------------------------------------
    // 3. Initial IP & Rate Quota Fetch
    // -------------------------------------------------------------
    async function fetchIPInfo() {
        try {
            const res = await fetch('/api/speedtest/ip');
            if (res.status === 403) {
                const data = await res.json();
                showError('Bot Guard Alert: ' + (data.reason || 'Automated client blocked.'));
                return;
            }
            if (!res.ok) throw new Error('Failed to resolve IP details');
            const data = await res.json();
            currentIPInfo = data;

            ipDisplay.textContent = data.ip;
            ispDisplay.textContent = data.isp || 'Broadband ISP';
            
            const loc = [data.city, data.country].filter(Boolean).join(', ');
            locationDisplay.textContent = loc || 'Global Internet';

            if (data.rate_limit) {
                quotaRemaining.textContent = data.rate_limit.remaining;
                if (!data.rate_limit.allowed) {
                    showError(`Hourly rate limit exceeded. You can run maximum 10 tests per hour.`);
                    startBtn.disabled = true;
                    startBtn.classList.add('opacity-50', 'cursor-not-allowed');
                }
            }
        } catch (err) {
            console.warn('IP lookup error:', err);
            ipDisplay.textContent = '127.0.0.1';
            ispDisplay.textContent = 'Local Connection';
            locationDisplay.textContent = 'Connected';
        }
    }

    // Click to copy IP
    ipDisplay.addEventListener('click', () => {
        if (ipDisplay.textContent && ipDisplay.textContent !== 'Detecting...') {
            navigator.clipboard.writeText(ipDisplay.textContent);
            const orig = ipDisplay.textContent;
            ipDisplay.textContent = 'Copied to clipboard!';
            setTimeout(() => { ipDisplay.textContent = orig; }, 1200);
        }
    });

    // -------------------------------------------------------------
    // 4. Ping and Jitter Diagnostic
    // -------------------------------------------------------------
    async function measurePingAndJitter() {
        phaseText.textContent = 'Testing latency and packet jitter...';
        gaugeLabel.textContent = 'PING';
        gaugeUnit.textContent = 'ms';

        const latencies = [];
        const iterations = 10;

        for (let i = 0; i < iterations; i++) {
            const start = performance.now();
            try {
                const res = await fetch('/api/speedtest/ping?t=' + Date.now(), { cache: 'no-store' });
                if (!res.ok) throw new Error('Ping failed');
                await res.json();
                const latency = performance.now() - start;
                latencies.push(latency);

                metricPing.textContent = latency.toFixed(1);
                barPing.style.width = Math.min((latency / 200) * 100, 100) + '%';
                targetGaugeSpeed = latency;

                await new Promise(r => setTimeout(r, 60));
            } catch (err) {
                console.error(err);
            }
        }

        if (latencies.length === 0) {
            throw new Error('Unable to communicate with latency server.');
        }

        const sum = latencies.reduce((a, b) => a + b, 0);
        finalPing = sum / latencies.length;
        metricPing.textContent = finalPing.toFixed(1);

        let jitterSum = 0;
        for (let i = 1; i < latencies.length; i++) {
            jitterSum += Math.abs(latencies[i] - latencies[i - 1]);
        }
        finalJitter = latencies.length > 1 ? (jitterSum / (latencies.length - 1)) : 0;
        metricJitter.textContent = finalJitter.toFixed(1);
        barJitter.style.width = Math.min((finalJitter / 50) * 100, 100) + '%';

        await new Promise(r => setTimeout(r, 350));
    }

    // -------------------------------------------------------------
    // 5. Download Throughput Diagnostic (Sliding Window & EMA)
    // -------------------------------------------------------------
    async function measureDownload() {
        phaseText.textContent = 'Testing download throughput (multi-stream)...';
        gaugeLabel.textContent = 'DOWNLOAD';
        gaugeUnit.textContent = 'Mbps';
        smoothedDownloadSpeed = 0;

        const testDurationMs = 8000;
        const startTime = performance.now();
        let totalBytes = 0;
        const streams = 4;
        let activeStreams = true;

        const downloadController = new AbortController();
        const timeoutId = setTimeout(() => {
            activeStreams = false;
            downloadController.abort();
        }, testDurationMs);

        // Sliding 1.2-second window buffer: [{ time, bytes }]
        const sampleWindow = [];
        const windowSizeMs = 1200;

        const readStream = async () => {
            while (activeStreams) {
                try {
                    const res = await fetch('/api/speedtest/download?size=30000000&r=' + Math.random(), {
                        signal: downloadController.signal,
                        cache: 'no-store'
                    });
                    if (!res.ok) return;

                    const reader = res.body.getReader();
                    while (activeStreams) {
                        const { done, value } = await reader.read();
                        if (done) break;
                        const len = value.byteLength;
                        totalBytes += len;
                        sampleWindow.push({ time: performance.now(), bytes: len });
                    }
                } catch (e) {
                    break;
                }
            }
        };

        // Smooth ticker updating every 150ms
        const updateInterval = setInterval(() => {
            const now = performance.now();
            const elapsed = now - startTime;

            // Discard samples older than windowSizeMs
            const cutoff = now - windowSizeMs;
            while (sampleWindow.length > 0 && sampleWindow[0].time < cutoff) {
                sampleWindow.shift();
            }

            // Calculate bytes inside sliding window
            let bytesInWindow = 0;
            for (let i = 0; i < sampleWindow.length; i++) {
                bytesInWindow += sampleWindow[i].bytes;
            }

            const activeWindowTimeSec = Math.min(elapsed, windowSizeMs) / 1000;
            if (activeWindowTimeSec > 0.2) {
                const instantMbps = (bytesInWindow * 8) / (activeWindowTimeSec * 1000000);

                // EMA smoothing (alpha = 0.28) eliminates TCP buffer jitter
                if (smoothedDownloadSpeed <= 0) {
                    smoothedDownloadSpeed = instantMbps;
                } else {
                    smoothedDownloadSpeed = (0.28 * instantMbps) + (0.72 * smoothedDownloadSpeed);
                }

                targetGaugeSpeed = smoothedDownloadSpeed;
                metricDownload.textContent = smoothedDownloadSpeed.toFixed(1);
                barDownload.style.width = Math.min((smoothedDownloadSpeed / 500) * 100, 100) + '%';

                if (throughputChart) {
                    throughputChart.data.labels.push('');
                    throughputChart.data.datasets[0].data.push(parseFloat(smoothedDownloadSpeed.toFixed(2)));
                    throughputChart.data.datasets[1].data.push(null);
                    throughputChart.update('none');
                }
            }
        }, 150);

        const streamPromises = [];
        for (let i = 0; i < streams; i++) {
            streamPromises.push(readStream());
        }

        await Promise.allSettled(streamPromises);
        clearTimeout(timeoutId);
        clearInterval(updateInterval);

        const totalElapsedSec = (performance.now() - startTime) / 1000;
        finalDownload = (totalBytes * 8) / (totalElapsedSec * 1000000);
        metricDownload.textContent = finalDownload.toFixed(1);
        targetGaugeSpeed = finalDownload;

        await new Promise(r => setTimeout(r, 500));
    }

    // -------------------------------------------------------------
    // 6. Upload Throughput Diagnostic (Progressive Chunks & EMA)
    // -------------------------------------------------------------
    async function measureUpload() {
        phaseText.textContent = 'Testing upload throughput...';
        gaugeLabel.textContent = 'UPLOAD';
        gaugeUnit.textContent = 'Mbps';
        smoothedUploadSpeed = 0;

        const testDurationMs = 8000;
        const startTime = performance.now();
        let totalUploadedBytes = 0;
        let activeUpload = true;

        // Progressive 512KB payload chunks for smooth incremental transfers
        const chunkSizeBytes = 512 * 1024;
        const chunkData = new Uint8Array(chunkSizeBytes);
        for (let i = 0; i < chunkSizeBytes; i += 1024) {
            chunkData[i] = Math.floor(Math.random() * 255);
        }

        const uploadController = new AbortController();
        const timeoutId = setTimeout(() => {
            activeUpload = false;
            uploadController.abort();
        }, testDurationMs);

        const uploadSampleWindow = [];
        const windowSizeMs = 1200;

        const updateInterval = setInterval(() => {
            const now = performance.now();
            const elapsed = now - startTime;

            const cutoff = now - windowSizeMs;
            while (uploadSampleWindow.length > 0 && uploadSampleWindow[0].time < cutoff) {
                uploadSampleWindow.shift();
            }

            let bytesInWindow = 0;
            for (let i = 0; i < uploadSampleWindow.length; i++) {
                bytesInWindow += uploadSampleWindow[i].bytes;
            }

            const activeWindowTimeSec = Math.min(elapsed, windowSizeMs) / 1000;
            if (activeWindowTimeSec > 0.2) {
                const instantMbps = (bytesInWindow * 8) / (activeWindowTimeSec * 1000000);

                if (smoothedUploadSpeed <= 0) {
                    smoothedUploadSpeed = instantMbps;
                } else {
                    smoothedUploadSpeed = (0.28 * instantMbps) + (0.72 * smoothedUploadSpeed);
                }

                targetGaugeSpeed = smoothedUploadSpeed;
                metricUpload.textContent = smoothedUploadSpeed.toFixed(1);
                barUpload.style.width = Math.min((smoothedUploadSpeed / 300) * 100, 100) + '%';

                if (throughputChart) {
                    throughputChart.data.labels.push('');
                    throughputChart.data.datasets[0].data.push(null);
                    throughputChart.data.datasets[1].data.push(parseFloat(smoothedUploadSpeed.toFixed(2)));
                    throughputChart.update('none');
                }
            }
        }, 150);

        // Upload worker with 512KB chunks
        const uploadWorker = async () => {
            while (activeUpload) {
                try {
                    const startChunk = performance.now();
                    const res = await fetch('/api/speedtest/upload', {
                        method: 'POST',
                        body: chunkData,
                        signal: uploadController.signal,
                        headers: { 'Content-Type': 'application/octet-stream' }
                    });
                    if (res.ok) {
                        totalUploadedBytes += chunkSizeBytes;
                        uploadSampleWindow.push({ time: performance.now(), bytes: chunkSizeBytes });
                    }
                } catch (e) {
                    break;
                }
            }
        };

        const workers = [uploadWorker(), uploadWorker(), uploadWorker(), uploadWorker()];
        await Promise.allSettled(workers);

        clearTimeout(timeoutId);
        clearInterval(updateInterval);

        const totalElapsedSec = (performance.now() - startTime) / 1000;
        finalUpload = (totalUploadedBytes * 8) / (totalElapsedSec * 1000000);
        metricUpload.textContent = finalUpload.toFixed(1);
        targetGaugeSpeed = finalUpload;

        await new Promise(r => setTimeout(r, 500));
    }

    // -------------------------------------------------------------
    // 7. Save Results & Complete Test
    // -------------------------------------------------------------
    async function saveResults() {
        phaseText.textContent = 'Finalizing and recording test results...';

        try {
            const payload = {
                download_mbps: parseFloat(finalDownload.toFixed(2)),
                upload_mbps: parseFloat(finalUpload.toFixed(2)),
                ping_ms: parseFloat(finalPing.toFixed(2)),
                jitter_ms: parseFloat(finalJitter.toFixed(2)),
                isp: currentIPInfo ? currentIPInfo.isp : 'Unknown',
                city: currentIPInfo ? currentIPInfo.city : '',
                country: currentIPInfo ? currentIPInfo.country : '',
                country_code: currentIPInfo ? currentIPInfo.country_code : ''
            };

            const res = await fetch('/api/speedtest/results', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });

            if (res.status === 429) {
                const data = await res.json();
                showError(data.error || 'Rate limit reached: Maximum 10 tests/hour per IP.');
                return;
            }

            if (res.status === 403) {
                showError('Bot Guard Alert: Access denied for automated requests.');
                return;
            }

            const data = await res.json();
            if (data.id) {
                lastSavedTestId = data.id;
            }
            if (data.remaining !== undefined) {
                quotaRemaining.textContent = data.remaining;
            }
        } catch (err) {
            console.error('Failed to log test results:', err);
        }

        let rating = 'Standard';
        if (finalDownload >= 200 && finalPing <= 15) {
            rating = 'Fiber Elite (Ideal for 8K streaming & competitive gaming)';
        } else if (finalDownload >= 50 && finalPing <= 30) {
            rating = 'Ultra Fast (Excellent for 4K streaming & video calls)';
        } else if (finalDownload >= 20) {
            rating = 'Good (HD streaming & everyday work)';
        } else {
            rating = 'Fair (Basic web browsing)';
        }
        connectionRating.textContent = 'Rating: ' + rating;

        gaugeLabel.textContent = 'DONE';
        phaseText.textContent = 'Diagnostic complete! View your results summary below.';
        targetGaugeSpeed = finalDownload;
        resultModal.classList.remove('hidden');
    }

    function showError(msg) {
        errorMessage.textContent = msg;
        errorAlert.classList.remove('hidden');
    }

    function resetUI() {
        errorAlert.classList.add('hidden');
        resultModal.classList.add('hidden');
        metricPing.textContent = '--';
        metricJitter.textContent = '--';
        metricDownload.textContent = '--';
        metricUpload.textContent = '--';
        barPing.style.width = '0%';
        barJitter.style.width = '0%';
        barDownload.style.width = '0%';
        barUpload.style.width = '0%';
        targetGaugeSpeed = 0;
        currentGaugeSpeed = 0;
        smoothedDownloadSpeed = 0;
        smoothedUploadSpeed = 0;

        if (throughputChart) {
            throughputChart.data.labels = [];
            throughputChart.data.datasets[0].data = [];
            throughputChart.data.datasets[1].data = [];
            throughputChart.update();
        }
    }

    // -------------------------------------------------------------
    // 8. Main Test Execution Orchestrator
    // -------------------------------------------------------------
    async function runSpeedTest() {
        if (isTesting) return;
        isTesting = true;
        startBtn.classList.add('hidden');
        resetUI();

        try {
            await measurePingAndJitter();
            await measureDownload();
            await measureUpload();
            await saveResults();
        } catch (err) {
            console.error('Speedtest aborted with error:', err);
            showError(err.message || 'Diagnostic failed. Please check your connection.');
            phaseText.textContent = 'Test aborted due to error.';
            startBtn.classList.remove('hidden');
        } finally {
            isTesting = false;
        }
    }

    // Share Button Event Listener
    const shareBtn = document.getElementById('share-btn');
    const shareBtnText = document.getElementById('share-btn-text');
    let lastSavedTestId = null;

    if (shareBtn) {
        shareBtn.addEventListener('click', async () => {
            const shareUrl = window.location.origin + '/?result=' + (lastSavedTestId || '');
            const shareText = `🚀 Speedtest Result:\n📥 Download: ${finalDownload.toFixed(1)} Mbps\n📤 Upload: ${finalUpload.toFixed(1)} Mbps\n📶 Ping: ${finalPing.toFixed(1)} ms | Jitter: ${finalJitter.toFixed(1)} ms\n\nView details: ${shareUrl}`;

            if (navigator.share) {
                try {
                    await navigator.share({
                        title: 'My Speedtest Result',
                        text: shareText,
                        url: shareUrl
                    });
                    return;
                } catch (e) {
                    // Fallback to clipboard
                }
            }

            navigator.clipboard.writeText(shareUrl).then(() => {
                const orig = shareBtnText ? shareBtnText.textContent : 'Share';
                if (shareBtnText) shareBtnText.textContent = 'Link Copied!';
                setTimeout(() => { if (shareBtnText) shareBtnText.textContent = orig; }, 2000);
            }).catch(() => {
                prompt('Copy shareable link:', shareUrl);
            });
        });
    }

    // Check if loading a shared test result (?result=123)
    async function checkSharedResult() {
        const urlParams = new URLSearchParams(window.location.search);
        const resultId = urlParams.get('result');
        if (!resultId) return;

        try {
            const res = await fetch('/api/speedtest/result?id=' + encodeURIComponent(resultId));
            if (!res.ok) return;
            const data = await res.json();

            lastSavedTestId = data.id;
            finalDownload = data.download_mbps;
            finalUpload = data.upload_mbps;
            finalPing = data.ping_ms;
            finalJitter = data.jitter_ms;

            metricPing.textContent = data.ping_ms.toFixed(1);
            metricJitter.textContent = data.jitter_ms.toFixed(1);
            metricDownload.textContent = data.download_mbps.toFixed(1);
            metricUpload.textContent = data.upload_mbps.toFixed(1);

            barPing.style.width = Math.min((data.ping_ms / 200) * 100, 100) + '%';
            barJitter.style.width = Math.min((data.jitter_ms / 50) * 100, 100) + '%';
            barDownload.style.width = Math.min((data.download_mbps / 500) * 100, 100) + '%';
            barUpload.style.width = Math.min((data.upload_mbps / 300) * 100, 100) + '%';

            targetGaugeSpeed = data.download_mbps;
            gaugeLabel.textContent = 'DOWNLOAD';

            ipDisplay.textContent = data.ip;
            ispDisplay.textContent = data.isp || 'Broadband ISP';
            const loc = [data.city, data.country].filter(Boolean).join(', ');
            locationDisplay.textContent = loc || 'Shared Result';

            phaseText.textContent = 'Viewing Shared Speedtest Result #' + data.id;
            const metaText = document.getElementById('result-meta-text');
            if (metaText) {
                const dateStr = new Date(data.timestamp).toLocaleString();
                metaText.textContent = `Recorded on ${dateStr} for ${data.ip} (${data.isp})`;
            }

            let rating = 'Standard';
            if (data.download_mbps >= 200 && data.ping_ms <= 15) {
                rating = 'Fiber Elite (Ideal for 8K streaming & competitive gaming)';
            } else if (data.download_mbps >= 50 && data.ping_ms <= 30) {
                rating = 'Ultra Fast (Excellent for 4K streaming & video calls)';
            } else if (data.download_mbps >= 20) {
                rating = 'Good (HD streaming & everyday work)';
            } else {
                rating = 'Fair (Basic web browsing)';
            }
            connectionRating.textContent = 'Rating: ' + rating;
            resultModal.classList.remove('hidden');
        } catch (e) {
            console.warn('Failed to load shared result:', e);
        }
    }

    // Event Listeners
    startBtn.addEventListener('click', runSpeedTest);
    restartBtn.addEventListener('click', () => {
        resultModal.classList.add('hidden');
        startBtn.classList.remove('hidden');
        runSpeedTest();
    });

    // Initialize Page
    initChart();
    animateGauge();
    fetchIPInfo();
    checkSharedResult();
});
