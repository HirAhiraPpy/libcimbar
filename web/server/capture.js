// Cimbar Web Decoder - Camera Capture and WebSocket Client

(function() {
  'use strict';

  // Configuration
  const WS_URL = `ws://${window.location.host}/ws`;
  const RECONNECT_DELAY = 2000;
  const MAX_QUEUE_SIZE = 30;

  // State
  let ws = null;
  let video = null;
  let canvas = null;
  let ctx = null;
  let frameCount = 0;
  let lastDecodeTime = 0;
  let isConnected = false;
  let isScanning = false;
  let frameQueue = [];
  let isProcessing = false;

  // DOM elements
  const scanFrame = document.getElementById('scan-frame');
  const statusText = document.getElementById('status-text');
  const progressContainer = document.getElementById('progress-container');
  const modeSelect = document.getElementById('mode-select');
  const errorMessage = document.getElementById('error-message');
  const fileComplete = document.getElementById('file-complete');
  const completeFilename = document.getElementById('complete-filename');
  const completeSize = document.getElementById('complete-size');

  // Initialize
  function init() {
    video = document.getElementById('video');
    canvas = document.createElement('canvas');
    ctx = canvas.getContext('2d', { willReadFrequently: true });

    setupCamera();
    setupWebSocket();
    setupEventListeners();
  }

  // Setup camera
  async function setupCamera() {
    try {
      statusText.textContent = 'Requesting camera access...';

      const constraints = {
        video: {
          width: { ideal: 1920 },
          height: { ideal: 1080 },
          facingMode: 'environment',
          frameRate: { ideal: 30 }
        },
        audio: false
      };

      const stream = await navigator.mediaDevices.getUserMedia(constraints);
      video.srcObject = stream;

      await new Promise((resolve) => {
        video.onloadedmetadata = () => {
          canvas.width = video.videoWidth;
          canvas.height = video.videoHeight;
          resolve();
        };
      });

      statusText.textContent = 'Camera ready';
      startCapture();
    } catch (err) {
      showError(`Camera error: ${err.message}`);
      statusText.textContent = 'Camera access denied';
    }
  }

  // Setup WebSocket
  function setupWebSocket() {
    try {
      ws = new WebSocket(WS_URL);
      ws.binaryType = 'arraybuffer';

      ws.onopen = () => {
        isConnected = true;
        statusText.textContent = 'Connected to server';
        hideError();
      };

      ws.onclose = () => {
        isConnected = false;
        statusText.textContent = 'Disconnected. Reconnecting...';
        setTimeout(setupWebSocket, RECONNECT_DELAY);
      };

      ws.onerror = (err) => {
        console.error('WebSocket error:', err);
        statusText.textContent = 'Connection error';
      };

      ws.onmessage = (event) => {
        try {
          const response = JSON.parse(event.data);
          handleServerResponse(response);
        } catch (err) {
          console.error('Failed to parse response:', err);
        }
      };
    } catch (err) {
      console.error('WebSocket connection failed:', err);
      setTimeout(setupWebSocket, RECONNECT_DELAY);
    }
  }

  // Handle server response
  function handleServerResponse(response) {
    console.log('Server response:', response);

    switch (response.type) {
      case 'decode':
        if (response.nodata) {
          scanFrame.classList.remove('active');
          scanFrame.classList.add('scanning');
        } else if (response.extracted) {
          scanFrame.classList.add('active');
          scanFrame.classList.remove('scanning');

          if (response.progress && response.progress.length > 0) {
            updateProgress(response.progress);
          }
        }
        break;

      case 'complete':
        if (response.success) {
          showFileComplete(response.filename, response.file_size);
          resetProgress();
        }
        break;

      case 'error':
        showError(response.error);
        break;
    }
  }

  // Update progress bars
  function updateProgress(progress) {
    progressContainer.innerHTML = '';

    progress.forEach((pct, idx) => {
      const bar = document.createElement('div');
      bar.className = 'progress-bar';

      const fill = document.createElement('div');
      fill.className = 'progress-fill';
      fill.style.width = `${pct}%`;

      bar.appendChild(fill);
      progressContainer.appendChild(bar);
    });
  }

  // Reset progress
  function resetProgress() {
    progressContainer.innerHTML = '';
  }

  // Start frame capture
  function startCapture() {
    if (video.requestVideoFrameCallback) {
      video.requestVideoFrameCallback(onVideoFrame);
    } else {
      // Fallback to requestAnimationFrame
      captureLoop();
    }
  }

  // Video frame callback
  function onVideoFrame(now, metadata) {
    captureFrame();
    video.requestVideoFrameCallback(onVideoFrame);
  }

  // Fallback capture loop
  function captureLoop() {
    captureFrame();
    requestAnimationFrame(captureLoop);
  }

  // Capture and send frame
  function captureFrame() {
    if (!isConnected || isProcessing) {
      return;
    }

    // Throttle to ~15 fps
    const now = performance.now();
    if (now - lastDecodeTime < 67) {
      return;
    }

    // Check queue size
    if (frameQueue.length >= MAX_QUEUE_SIZE) {
      frameQueue.shift(); // Drop oldest frame
    }

    frameQueue.push(true);
    lastDecodeTime = now;

    // Draw video frame to canvas
    ctx.drawImage(video, 0, 0, canvas.width, canvas.height);

    // Get pixel data
    const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height);

    // Send frame to server
    sendFrame(imageData);
  }

  // Send frame to server
  function sendFrame(imageData) {
    if (!isConnected || ws.readyState !== WebSocket.OPEN) {
      return;
    }

    // Frame format: [format:1][mode:1][width:2][height:2][pixels:...]
    const width = imageData.width;
    const height = imageData.height;
    const pixels = imageData.data;

    // Use RGBA format (format code = 4)
    const format = 4;
    const mode = parseInt(modeSelect.value === 'Auto' ? '0' : '1');

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
    view.setUint16(2, width, true); // little-endian
    view.setUint16(4, height, true);

    // Write pixel data
    const pixelView = new Uint8Array(buffer, headerSize);
    pixelView.set(pixels);

    // Send
    ws.send(buffer);

    frameCount++;
    isProcessing = true;

    // Reset processing flag after a short delay
    setTimeout(() => {
      isProcessing = false;
    }, 10);
  }

  // Setup event listeners
  function setupEventListeners() {
    modeSelect.addEventListener('change', () => {
      statusText.textContent = `Mode: ${modeSelect.value}`;
    });

    // Handle visibility change (pause capture when tab is hidden)
    document.addEventListener('visibilitychange', () => {
      if (document.hidden) {
        isScanning = false;
      } else {
        isScanning = true;
      }
    });

    // Handle orientation change
    window.addEventListener('orientationchange', () => {
      setTimeout(() => {
        canvas.width = video.videoWidth;
        canvas.height = video.videoHeight;
      }, 100);
    });
  }

  // Show error
  function showError(message) {
    errorMessage.textContent = message;
    errorMessage.style.display = 'block';

    setTimeout(() => {
      hideError();
    }, 5000);
  }

  // Hide error
  function hideError() {
    errorMessage.style.display = 'none';
  }

  // Show file complete
  function showFileComplete(filename, size) {
    completeFilename.textContent = filename;
    completeSize.textContent = formatFileSize(size);
    fileComplete.style.display = 'block';

    // Flash the scan frame
    scanFrame.style.borderColor = '#0f0';
    scanFrame.style.boxShadow = '0 0 50px rgba(0, 255, 0, 0.8)';

    setTimeout(() => {
      fileComplete.style.display = 'none';
      scanFrame.style.borderColor = '#0f0';
      scanFrame.style.boxShadow = '0 0 20px rgba(0, 255, 0, 0.5)';
    }, 3000);
  }

  // Format file size
  function formatFileSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
  }

  // Start when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
