// Cimbar Web Decoder - Camera Capture and WebSocket Client

(function () {
    "use strict";

    // Configuration
    const WS_PROTOCOL = window.location.protocol === "https:" ? "wss:" : "ws:";
    const WS_URL = `${WS_PROTOCOL}//${window.location.host}/ws`;
    const RECONNECT_DELAY = 2000;

    // Capture settings
    const CAPTURE_INTERVAL = 100; // ~10 fps
    const CROP_SIZE = 1024; // Square crop for cimbar (1024x1024 is the standard cimbar size)

    // State
    let ws = null;
    let video = null;
    let canvas = null;
    let cropCanvas = null;
    let ctx = null;
    let cropCtx = null;
    let frameCount = 0;
    let lastCaptureTime = 0;
    let isConnected = false;
    let isScanning = false;
    let captureIntervalId = null;
    let isProcessing = false;
    let serverResponseCount = 0;
    let lastServerResponse = 0;
    let isScanActive = false; // Whether scanning is enabled by user
    let currentResolution = { width: 1280, height: 720 };
    let pingIntervalId = null; // Heartbeat ping interval

    // DOM elements
    const scanFrame = document.getElementById("scan-frame");
    const cropOverlay = document.getElementById("crop-overlay");
    const statusText = document.getElementById("status-text");
    const fileInfo = document.getElementById("file-info");
    const progressContainer = document.getElementById("progress-container");
    const scanBtn = document.getElementById("scan-btn");
    const errorMessage = document.getElementById("error-message");
    const fileComplete = document.getElementById("file-complete");
    const completeFilename = document.getElementById("complete-filename");
    const completeSize = document.getElementById("complete-size");
    const connectionStatus = document.getElementById("connection-status");
    const scanIndicator = document.getElementById("scan-indicator");
    const scanIndicatorText = document.getElementById("scan-indicator-text");

    // Sidebar elements
    const settingsSidebar = document.getElementById("settings-sidebar");
    const settingsToggle = document.getElementById("settings-toggle");
    const settingsClose = document.getElementById("settings-close");
    const sidebar = document.getElementById("sidebar");
    const filesToggle = document.getElementById("files-toggle");
    const sidebarClose = document.getElementById("sidebar-close");
    const completedFilesList = document.getElementById("completed-files-list");

    // Settings elements (in sidebar)
    const modeSelect = document.getElementById("mode-select");
    const resolutionSelect = document.getElementById("resolution-select");

    // Completed files list
    let completedFiles = [];

    // Check if we're in a secure context
    function isSecureContext() {
        return (
            window.isSecureContext ||
            window.location.protocol === "https:" ||
            window.location.hostname === "localhost" ||
            window.location.hostname === "127.0.0.1"
        );
    }

    // Check HTTPS and show warning if needed
    function checkSecureContext() {
        if (!isSecureContext()) {
            showError(
                "警告：摄像头需要 HTTPS 环境！\n\n" +
                    "当前连接不是 HTTPS，浏览器可能会阻止摄像头访问。\n\n" +
                    "解决方案：\n" +
                    "1. 使用 localhost 访问（允许摄像头）\n" +
                    "2. 配置 HTTPS（推荐）\n" +
                    "3. 在 Chrome 中访问 chrome://flags/#unsafely-treat-insecure-origin-as-secure 添加此站点",
            );
            return false;
        }
        return true;
    }

    // Initialize
    function init() {
        video = document.getElementById("video");
        canvas = document.createElement("canvas");
        ctx = canvas.getContext("2d", { willReadFrequently: true });

        cropCanvas = document.createElement("canvas");
        cropCanvas.width = CROP_SIZE;
        cropCanvas.height = CROP_SIZE;
        cropCtx = cropCanvas.getContext("2d");

        // Check secure context first
        checkSecureContext();

        setupEventListeners();
        setupCamera();
        setupWebSocket();

        // Load completed files from server
        loadCompletedFiles();
    }

    // Setup camera with selected resolution
    async function setupCamera() {
        try {
            statusText.textContent = "正在请求摄像头权限...";
            updateScanIndicator("摄像头加载中", "stopped");

            // Parse selected resolution
            const [width, height] = resolutionSelect.value
                .split("x")
                .map(Number);
            currentResolution = { width, height };

            if (
                !navigator.mediaDevices ||
                !navigator.mediaDevices.getUserMedia
            ) {
                throw new Error("您的浏览器不支持摄像头 API");
            }

            const constraints = {
                video: {
                    width: { ideal: width },
                    height: { ideal: height },
                    facingMode: { ideal: "environment" },
                    frameRate: { ideal: 30 },
                },
                audio: false,
            };

            const stream =
                await navigator.mediaDevices.getUserMedia(constraints);
            video.srcObject = stream;

            // Wait for video to load
            await new Promise((resolve) => {
                video.onloadedmetadata = () => {
                    canvas.width = video.videoWidth;
                    canvas.height = video.videoHeight;
                    console.log(
                        `Video loaded: ${canvas.width}x${canvas.height}`,
                    );

                    // Update crop overlay size (center square)
                    updateCropOverlay();
                    resolve();
                };
                setTimeout(resolve, 3000);
            });

            if (video.readyState >= 2) {
                statusText.textContent = '摄像头就绪，点击"开始识别"';
                updateScanIndicator("已就绪", "stopped");
            } else {
                throw new Error("视频流加载失败");
            }
        } catch (err) {
            console.error("Camera error:", err);
            let errorMsg = `摄像头错误：${err.message}`;

            if (
                err.name === "NotAllowedError" ||
                err.name === "PermissionDeniedError"
            ) {
                errorMsg += "\n\n请允许摄像头权限后刷新页面";
            } else if (
                err.name === "NotFoundError" ||
                err.name === "DevicesNotFoundError"
            ) {
                errorMsg += "\n\n未找到摄像头设备";
            } else if (
                err.name === "NotReadableError" ||
                err.name === "TrackStartError"
            ) {
                errorMsg += "\n\n摄像头可能正在被其他程序使用";
            } else if (!isSecureContext()) {
                errorMsg +=
                    "\n\n非 HTTPS 环境可能无法使用摄像头，请使用 localhost 或配置 HTTPS";
            }

            showError(errorMsg);
            statusText.textContent = "摄像头不可用";
            updateScanIndicator("错误", "stopped");
        }
    }

    // Update crop overlay to show square crop area
    function updateCropOverlay() {
        const videoWidth = video.videoWidth || 1280;
        const videoHeight = video.videoHeight || 720;
        const size = Math.min(videoWidth, videoHeight);

        // Calculate display size (CSS pixels)
        const containerWidth = window.innerWidth;
        const containerHeight = window.innerHeight;
        const videoRatio = videoWidth / videoHeight;
        const containerRatio = containerWidth / containerHeight;

        let displayWidth, displayHeight;
        if (videoRatio > containerRatio) {
            displayHeight = containerHeight;
            displayWidth = displayHeight * videoRatio;
        } else {
            displayWidth = containerWidth;
            displayHeight = displayWidth / videoRatio;
        }

        // Square crop size in display pixels - match scan-frame size
        const scanFrameSize = Math.min(
            window.innerWidth * 0.85,
            window.innerHeight * 0.85,
            512,
        );
        const cropDisplaySize = scanFrameSize * 0.95; // Slightly smaller than scan-frame

        cropOverlay.style.width = `${cropDisplaySize}px`;
        cropOverlay.style.height = `${cropDisplaySize}px`;

        updateDebugStats();
    }

    // Setup WebSocket
    function setupWebSocket() {
        console.log("Connecting to WebSocket:", WS_URL);
        statusText.textContent = "正在连接服务器...";

        try {
            ws = new WebSocket(WS_URL);
            ws.binaryType = "arraybuffer";

            ws.onopen = () => {
                console.log("WebSocket connected!");
                isConnected = true;
                updateConnectionStatus("已连接");
                if (isScanActive) {
                    statusText.textContent = '就绪，点击"开始识别"';
                }
                hideError();

                // Start heartbeat ping
                startPing();
            };

            ws.onclose = (event) => {
                console.log("WebSocket closed:", event.code, event.reason);
                isConnected = false;
                updateConnectionStatus("已断开");
                if (
                    statusText.textContent.indexOf("摄像头") === -1 &&
                    statusText.textContent.indexOf("就绪") === -1
                ) {
                    statusText.textContent = "已断开，重连中...";
                }
                // Stop ping
                stopPing();
                setTimeout(setupWebSocket, RECONNECT_DELAY);
            };

            ws.onerror = (err) => {
                console.error("WebSocket error:", err);
                updateConnectionStatus("错误");
                statusText.textContent = "连接错误";
                showError(
                    "WebSocket 连接失败\n\n请检查：\n1. 服务器是否运行正常\n2. Nginx WebSocket 代理配置是否正确\n3. 防火墙设置",
                );
            };

            ws.onmessage = (event) => {
                try {
                    const response = JSON.parse(event.data);
                    handleServerResponse(response);
                } catch (err) {
                    console.error("Failed to parse response:", err);
                }
            };
        } catch (err) {
            console.error("WebSocket connection failed:", err);
            setTimeout(setupWebSocket, RECONNECT_DELAY);
        }
    }

    // Update connection status display
    function updateConnectionStatus(status) {
        if (connectionStatus) {
            connectionStatus.textContent = status;
            connectionStatus.className =
                status === "已连接" ? "connected" : "disconnected";
        }
    }

    // Heartbeat ping functions
    function startPing() {
        stopPing(); // Clear any existing interval
        pingIntervalId = setInterval(() => {
            if (ws && ws.readyState === WebSocket.OPEN) {
                // Send empty message as heartbeat
                ws.send(JSON.stringify({ type: "ping" }));
            }
        }, 30000); // Send ping every 30 seconds
    }

    function stopPing() {
        if (pingIntervalId) {
            clearInterval(pingIntervalId);
            pingIntervalId = null;
        }
    }

    // Update scan indicator
    function updateScanIndicator(text, state) {
        if (scanIndicatorText) {
            scanIndicatorText.textContent = text;
        }
        scanIndicator.className = state;
    }

    // Toggle scanning
    function toggleScanning() {
        isScanActive = !isScanActive;

        if (isScanActive) {
            scanBtn.textContent = "停止识别";
            scanBtn.className = "btn btn-danger";
            updateScanIndicator("扫描中", "active");
            statusText.textContent = "正在扫描 Cimbar...";

            // Request decoder reset before starting
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({ type: "reset_decoder" }));
            }
            startCapture();
        } else {
            scanBtn.textContent = "开始识别";
            scanBtn.className = "btn btn-primary";
            updateScanIndicator("已停止", "stopped");
            statusText.textContent = "已停止";
            stopCapture();
        }
    }

    // Handle server response
    function handleServerResponse(response) {
        console.log("Server response:", response);
        serverResponseCount++;
        lastServerResponse = performance.now();

        switch (response.type) {
            case "decode":
                console.log(
                    "Decode response - bytes:",
                    response.bytes,
                    "file_size:",
                    response.file_size,
                    "bytes_recv:",
                    response.bytes_recv,
                );

                if (response.nodata) {
                    // Server received frame but no data extracted
                    scanFrame.classList.remove("active");
                    scanFrame.classList.add("scanning");
                    cropOverlay.classList.remove("active");
                    cropOverlay.classList.add("scanning");
                    // 只更新状态灯颜色（黄色），不更新文字
                    scanIndicator.className = "scanning";
                    fileInfo.textContent = "";
                } else if (response.extracted) {
                    // Server successfully extracted data
                    scanFrame.classList.add("active");
                    scanFrame.classList.remove("scanning");
                    cropOverlay.classList.add("active");
                    cropOverlay.classList.remove("scanning");
                    updateScanIndicator("解码中...", "active");

                    // Show filename and file info
                    if (response.filename) {
                        fileInfo.textContent = `文件：${response.filename} | 总大小：${formatFileSize(response.file_size)}`;
                    } else if (response.file_size > 0) {
                        fileInfo.textContent = `总大小：${formatFileSize(response.file_size)}`;
                    }

                    // Show bytes received progress
                    const bytesRecv =
                        response.bytes_decoded || response.bytes_recv || 0;
                    if (response.file_size > 0) {
                        const percent = Math.round(
                            (bytesRecv * 100) / response.file_size,
                        );
                        updateStatusText(
                            `已接收：${formatFileSize(bytesRecv)} / ${formatFileSize(response.file_size)} (${percent}%)`,
                        );
                    } else {
                        updateStatusText(`解码成功！${response.bytes} 字节`);
                    }

                    // Handle duplicate frame (file already decoded)
                    if (response.is_duplicate) {
                        updateStatusText("文件已完成，重复帧已忽略");
                    }

                    if (response.progress && response.progress.length > 0) {
                        updateProgress(
                            response.progress,
                            response.file_size,
                            bytesRecv,
                        );
                    }
                } else {
                    updateStatusText("解码中...");
                    fileInfo.textContent = "";
                }
                break;

            case "complete":
                console.log(
                    "File complete!",
                    response.filename,
                    response.file_size,
                );
                if (response.success) {
                    showFileComplete(response.filename, response.file_size);
                    resetProgress();
                    fileInfo.textContent = "";
                    updateStatusText("文件已保存！可以点击「停止识别」然后重新开始接收下一个文件");
                    updateScanIndicator("完成", "active");

                    // Add to completed files list
                    completedFiles.push({
                        filename: response.filename,
                        fileSize: response.file_size,
                        completedAt: new Date().toISOString()
                    });

                    // Don't auto-stop, let user decide when to stop
                    // User can click "停止识别" to stop, then "开始识别" to receive next file
                }
                break;

            case "file_complete":
                // Server notification that file is complete
                console.log("Server notified file complete");
                fileInfo.textContent = "";
                updateStatusText("文件已完成！可以开始接收下一个文件");
                updateScanIndicator("完成", "active");
                // Don't auto-stop scanning
                break;

            case "error":
                console.error("Server error:", response.error);
                showError(response.error);
                updateStatusText("错误：" + response.error);
                updateScanIndicator("错误", "stopped");
                break;

            default:
                console.log("Unknown response type:", response.type);
        }
    }

    // Update status text
    function updateStatusText(text) {
        if (statusText) {
            statusText.textContent = text;
        }
    }

    // Update progress bars
    function updateProgress(progress, fileSize, bytesRecv) {
        progressContainer.innerHTML = "";

        // Show progress bars
        if (progress && progress.length > 0) {
            progress.forEach((pct, idx) => {
                const bar = document.createElement("div");
                bar.className = "progress-bar";

                const fill = document.createElement("div");
                fill.className = "progress-fill";
                // Ensure pct is normalized to 0-1 range, then convert to percentage
                // Backend returns decimal (0.0-1.0), not percentage (0-100)
                let normalizedPct = pct;
                if (pct > 1) {
                    // If it looks like a percentage (e.g., 59.02), normalize it
                    normalizedPct = pct / 100;
                }
                const width = Math.min(100, Math.max(0, normalizedPct * 100));
                fill.style.width = `${width}%`;

                bar.appendChild(fill);
                progressContainer.appendChild(bar);
            });
        }

        // Show total size and progress info (only once)
        if (fileSize > 0) {
            const progressInfo = document.createElement("div");
            progressInfo.className = "progress-info";
            progressInfo.style.cssText =
                "font-size: 12px; color: #aaa; margin-top: 5px; text-align: center;";
            const actualBytesRecv = bytesRecv || 0;
            const percent = fileSize > 0 ? Math.round((actualBytesRecv * 100) / fileSize) : 0;
            progressInfo.textContent = `${formatFileSize(actualBytesRecv)} / ${formatFileSize(fileSize)} (${percent}%)`;
            progressContainer.appendChild(progressInfo);
        }
    }

    // Reset progress
    function resetProgress() {
        progressContainer.innerHTML = "";
        if (fileInfo) {
            fileInfo.textContent = "";
        }
    }

    // Start frame capture
    function startCapture() {
        if (
            !isScanActive ||
            !isConnected ||
            !ws ||
            ws.readyState !== WebSocket.OPEN
        ) {
            return;
        }

        // Clear any existing interval
        if (captureIntervalId) {
            clearInterval(captureIntervalId);
        }

        // Use setInterval for consistent frame capture
        captureIntervalId = setInterval(captureFrame, CAPTURE_INTERVAL);
        console.log("Capture started");
    }

    // Stop frame capture
    function stopCapture() {
        if (captureIntervalId) {
            clearInterval(captureIntervalId);
            captureIntervalId = null;
        }
    }

    // Capture and send cropped frame
    function captureFrame() {
        // Check if we should capture
        if (!isScanActive || !isConnected || ws.readyState !== WebSocket.OPEN) {
            return;
        }

        if (isProcessing) {
            return;
        }

        // Check if video is ready
        if (video.readyState < 2) {
            return;
        }

        const now = performance.now();
        if (now - lastCaptureTime < CAPTURE_INTERVAL - 10) {
            return;
        }

        lastCaptureTime = now;

        try {
            // Draw video frame to main canvas
            ctx.drawImage(video, 0, 0, canvas.width, canvas.height);

            // Crop center square region
            const videoWidth = canvas.width;
            const videoHeight = canvas.height;
            const size = Math.min(videoWidth, videoHeight);
            const offsetX = (videoWidth - size) / 2;
            const offsetY = (videoHeight - size) / 2;

            // Draw cropped region to crop canvas
            cropCtx.drawImage(
                canvas,
                offsetX,
                offsetY,
                size,
                size, // Source (cropped center)
                0,
                0,
                CROP_SIZE,
                CROP_SIZE, // Destination (1024x1024)
            );

            // Get cropped pixel data
            const imageData = cropCtx.getImageData(0, 0, CROP_SIZE, CROP_SIZE);

            // Send cropped frame to server
            sendFrame(imageData, CROP_SIZE, CROP_SIZE);

            frameCount++;
            isProcessing = true;

            // Update stats
            updateDebugStats();

            // Reset processing flag after a short delay
            setTimeout(() => {
                isProcessing = false;
            }, 50);
        } catch (err) {
            console.error("Capture error:", err);
        }
    }

    // Send frame to server
    function sendFrame(imageData, width, height) {
        if (!isConnected || ws.readyState !== WebSocket.OPEN) {
            return;
        }

        const pixels = imageData.data;

        // Use RGBA format (format code = 4)
        const format = 4;
        const mode = modeSelect.value === "Auto" ? 0 : 1;

        // Calculate buffer size
        const headerSize = 6;
        const pixelSize = width * height * 4;
        const bufferSize = headerSize + pixelSize;

        // Create ArrayBuffer
        const buffer = new ArrayBuffer(bufferSize);
        const view = new DataView(buffer);

        // Write header
        view.setUint8(0, format);
        view.setUint8(1, mode);
        view.setUint16(2, width, true);
        view.setUint16(4, height, true);

        // Write pixel data
        const pixelView = new Uint8Array(buffer, headerSize);
        pixelView.set(pixels);

        try {
            ws.send(buffer);

            if (frameCount % 10 === 0) {
                console.log(
                    `Frame ${frameCount}: Sent ${width}x${height} (cropped)`,
                );
            }
        } catch (err) {
            console.error("Send error:", err);
        }
    }

    // Setup event listeners
    function setupEventListeners() {
        // Scan button
        scanBtn.addEventListener("click", toggleScanning);

        // Settings sidebar toggle
        if (settingsToggle) {
            settingsToggle.addEventListener("click", () => {
                settingsSidebar.classList.toggle("open");
            });
        }

        // Settings sidebar close
        if (settingsClose) {
            settingsClose.addEventListener("click", () => {
                settingsSidebar.classList.remove("open");
            });
        }

        // Files sidebar toggle
        if (filesToggle) {
            filesToggle.addEventListener("click", () => {
                sidebar.classList.toggle("open");
                if (sidebar.classList.contains("open")) {
                    updateCompletedFilesList();
                }
            });
        }

        // Sidebar close
        if (sidebarClose) {
            sidebarClose.addEventListener("click", () => {
                sidebar.classList.remove("open");
            });
        }

        // Resolution change - reload camera
        resolutionSelect.addEventListener("change", () => {
            console.log("Resolution changed to:", resolutionSelect.value);
            statusText.textContent = "切换分辨率...";
            stopCapture();
            isScanActive = false;
            scanBtn.textContent = "开始识别";
            scanBtn.className = "btn btn-primary";
            setupCamera();
        });

        // Mode change
        modeSelect.addEventListener("change", () => {
            console.log("Mode changed to:", modeSelect.value);
        });

        // Handle visibility change
        document.addEventListener("visibilitychange", () => {
            if (document.hidden) {
                stopCapture();
            } else {
                if (isScanActive && isConnected && video.readyState >= 2) {
                    startCapture();
                }
            }
        });

        // Handle orientation change
        window.addEventListener("orientationchange", () => {
            setTimeout(() => {
                updateCropOverlay();
                if (video.videoWidth && video.videoHeight) {
                    canvas.width = video.videoWidth;
                    canvas.height = video.videoHeight;
                }
            }, 100);
        });

        // Handle page unload
        window.addEventListener("beforeunload", () => {
            stopCapture();
            stopPing();
            if (ws) {
                ws.close();
            }
            if (video.srcObject) {
                const tracks = video.srcObject.getTracks();
                tracks.forEach((track) => track.stop());
            }
        });
    }

    // Update debug stats
    function updateDebugStats() {
        const framesEl = document.getElementById("stat-frames");
        const responsesEl = document.getElementById("stat-responses");
        const cropEl = document.getElementById("stat-crop");

        if (framesEl) framesEl.textContent = frameCount.toString();
        if (responsesEl)
            responsesEl.textContent = serverResponseCount.toString();
        if (cropEl) cropEl.textContent = `${CROP_SIZE}x${CROP_SIZE}`;
    }

    // Show error
    function showError(message) {
        errorMessage.textContent = message;
        errorMessage.style.display = "block";
        errorMessage.style.whiteSpace = "pre-line";
    }

    // Hide error
    function hideError() {
        errorMessage.style.display = "none";
    }

    // Show file complete
    function showFileComplete(filename, size) {
        completeFilename.textContent = filename;
        completeSize.textContent = formatFileSize(size);
        fileComplete.style.display = "block";

        // Flash the scan frame
        scanFrame.style.borderColor = "#0f0";
        scanFrame.style.boxShadow = "0 0 50px rgba(0, 255, 0, 0.8)";
        cropOverlay.style.borderColor = "rgba(0, 255, 0, 0.9)";

        setTimeout(() => {
            fileComplete.style.display = "none";
            scanFrame.style.borderColor = "#0f0";
            scanFrame.style.boxShadow = "0 0 20px rgba(0, 255, 0, 0.5)";
            cropOverlay.style.borderColor = "rgba(0, 255, 0, 0.8)";
        }, 3000);
    }

    // Format file size
    function formatFileSize(bytes) {
        if (bytes < 1024) return bytes + " B";
        if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
        return (bytes / (1024 * 1024)).toFixed(1) + " MB";
    }

    // Update completed files list in sidebar
    function updateCompletedFilesList() {
        if (!completedFilesList) return;

        if (completedFiles.length === 0) {
            completedFilesList.innerHTML = '<div id="empty-files-msg">暂无已完成文件</div>';
            return;
        }

        completedFilesList.innerHTML = "";
        completedFiles.forEach((file, idx) => {
            const item = document.createElement("div");
            item.className = "completed-file-item";

            const filenameDiv = document.createElement("div");
            filenameDiv.className = "filename";
            filenameDiv.textContent = file.filename;

            const metaDiv = document.createElement("div");
            metaDiv.className = "meta";
            metaDiv.innerHTML = `<span>${formatFileSize(file.fileSize)}</span><span>${new Date(file.completedAt).toLocaleTimeString()}</span>`;

            const actionsDiv = document.createElement("div");
            actionsDiv.className = "file-actions";

            const downloadBtn = document.createElement("button");
            downloadBtn.textContent = "下载";
            downloadBtn.addEventListener("click", (e) => {
                e.stopPropagation();
                downloadFile(file.filename);
            });

            actionsDiv.appendChild(downloadBtn);
            item.appendChild(filenameDiv);
            item.appendChild(metaDiv);
            item.appendChild(actionsDiv);

            completedFilesList.appendChild(item);
        });
    }

    // Download file
    function downloadFile(filename) {
        const url = `/api/download?file=${encodeURIComponent(filename)}`;
        window.open(url, "_blank");
    }

    // Load completed files from server on init
    async function loadCompletedFiles() {
        try {
            const response = await fetch("/api/files");
            if (response.ok) {
                const files = await response.json();
                if (files && files.length > 0) {
                    completedFiles = files.map(f => ({
                        filename: f.filename,
                        fileSize: f.file_size,
                        completedAt: f.completed_at
                    }));
                }
            }
        } catch (err) {
            console.error("Failed to load completed files:", err);
        }
    }

    // Start when DOM is ready
    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", init);
    } else {
        init();
    }
})();
