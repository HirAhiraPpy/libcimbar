/**
 * WASM 解密模块
 * 使用 PBKDF2-HMAC-SHA256 派生密钥，AES-256-GCM 解密 WASM 数据
 */

const WasmdDecrypt = (function() {
  // 加密参数（需要与加密脚本一致）
  const PBKDF2_ITERATIONS = 100000;
  const KEY_LENGTH = 256; // bits
  const SALT_LENGTH = 16; // bytes
  const IV_LENGTH = 12;   // bytes (GCM standard)

  // 从加密文件头部提取元数据
  // 格式：[salt:16][iv:12][encrypted_data:...][tag:16]
  function parseEncryptedData(base64Data) {
    const binaryData = atob(base64Data);
    const data = new Uint8Array(binaryData.length);
    for (let i = 0; i < binaryData.length; i++) {
      data[i] = binaryData.charCodeAt(i);
    }

    if (data.length < SALT_LENGTH + IV_LENGTH + 16) {
      throw new Error('加密数据格式错误：数据长度不足');
    }

    const salt = data.slice(0, SALT_LENGTH);
    const iv = data.slice(SALT_LENGTH, SALT_LENGTH + IV_LENGTH);
    const encryptedDataWithTag = data.slice(SALT_LENGTH + IV_LENGTH);

    return { salt, iv, encryptedDataWithTag };
  }

  // 从口令派生密钥
  async function deriveKey(password, salt) {
    const encoder = new TextEncoder();
    const passwordBuffer = encoder.encode(password);

    const keyMaterial = await crypto.subtle.importKey(
      'raw',
      passwordBuffer,
      'PBKDF2',
      false,
      ['deriveKey']
    );

    const key = await crypto.subtle.deriveKey(
      {
        name: 'PBKDF2',
        salt: salt,
        iterations: PBKDF2_ITERATIONS,
        hash: 'SHA-256'
      },
      keyMaterial,
      { name: 'AES-GCM', length: KEY_LENGTH },
      false,
      ['decrypt']
    );

    return key;
  }

  // AES-GCM 解密
  async function decrypt(encryptedData, key, iv) {
    try {
      const decrypted = await crypto.subtle.decrypt(
        {
          name: 'AES-GCM',
          iv: iv
        },
        key,
        encryptedData
      );

      return new Uint8Array(decrypted);
    } catch (err) {
      console.error('解密失败:', err);
      throw new Error('口令错误或数据损坏');
    }
  }

  // 主解密函数
  async function decryptWasm(encryptedBase64, password) {
    try {
      const { salt, iv, encryptedDataWithTag } = parseEncryptedData(encryptedBase64);
      const key = await deriveKey(password, salt);
      const decrypted = await decrypt(encryptedDataWithTag, key, iv);
      return decrypted;
    } catch (err) {
      console.error('WASM 解密错误:', err);
      throw err;
    }
  }

  // 加载并初始化 WASM 模块
  async function loadWasmModule(wasmBytes) {
    const { instance, module } = await WebAssembly.instantiate(wasmBytes);
    return { instance, module };
  }

  return {
    decryptWasm,
    loadWasmModule
  };
})();

// 全局状态
let g_wasmModule = null;
let g_onWasmReady = null;
let g_wasmLoaded = false;

/**
 * 初始化密码验证和 WASM 加载
 * @param {string} encryptedBase64 - Base64 编码的加密 WASM 数据（可选）
 * @param {function} onReady - WASM 加载完成后的回调函数
 */
function initPasswordPrompt(encryptedBase64, onReady) {
  g_onWasmReady = onReady;

  const modal = document.getElementById('password-modal');
  const form = document.getElementById('password-form');
  const passwordInput = document.getElementById('password-input');
  const errorDiv = document.getElementById('password-error');
  const loadingSpinner = document.getElementById('decrypt-loading');
  const decryptStatus = document.getElementById('decrypt-status');

  // 保存加密数据
  if (encryptedBase64) {
    window.ENCRYPTED_WASM_DATA = encryptedBase64;
  }

  // 显示密码输入框
  modal.classList.add('visible');

  form.addEventListener('submit', async function(e) {
    e.preventDefault();

    const password = passwordInput.value;
    if (!password) {
      errorDiv.textContent = '请输入口令';
      return;
    }

    try {
      // 显示加载状态
      form.style.display = 'none';
      loadingSpinner.classList.add('visible');
      decryptStatus.style.display = 'block';

      // 尝试加载加密的 WASM
      await loadEncryptedWasm(password);

      // 解密成功，关闭模态框
      modal.classList.remove('visible');
      g_wasmLoaded = true;

      // 调用回调
      if (g_onWasmReady) {
        g_onWasmReady();
      }

    } catch (err) {
      console.error('WASM 加载失败:', err);
      errorDiv.textContent = err.message || '口令错误，请重试';
      form.style.display = 'block';
      loadingSpinner.classList.remove('visible');
      decryptStatus.style.display = 'none';
      passwordInput.value = '';
      passwordInput.focus();
    }
  });

  passwordInput.focus();
}

/**
 * 从外部加载并解密 WASM 模块
 */
async function loadEncryptedWasm(password) {
  let encryptedBase64;

  // 优先使用内嵌的数据
  if (window.ENCRYPTED_WASM_DATA) {
    console.log('使用内嵌的加密 WASM 数据');
    encryptedBase64 = window.ENCRYPTED_WASM_DATA;
  } else {
    // 回退：尝试加载外部 .enc 文件
    console.log('尝试加载外部加密 WASM 文件...');
    try {
      const response = await fetch('cimbar_js.wasm.enc');
      if (!response.ok) {
        throw new Error('无法加载加密 WASM 文件');
      }
      encryptedBase64 = await response.text();
    } catch (fetchErr) {
      console.error('加载外部文件失败:', fetchErr);
      throw new Error('无法找到加密的 WASM 文件');
    }
  }

  // 解密 WASM
  console.log('开始解密 WASM...');
  const decryptedWasm = await WasmdDecrypt.decryptWasm(encryptedBase64, password);
  console.log('WASM 解密成功，大小:', decryptedWasm.length, 'bytes');

  // 加载 WASM 模块
  const { instance, module } = await WasmdDecrypt.loadWasmModule(decryptedWasm);
  console.log('WASM 模块实例化成功');

  // 设置全局 Module 对象（兼容 emscripten 运行时）
  window.Module = window.Module || {};

  // 复制导出函数到 Module
  const exports = instance.exports;
  for (let key in exports) {
    window.Module[key] = exports[key];
  }

  // 设置 HEAP 缓冲区引用
  window.Module.HEAPU8 = exports.memory ? new Uint8Array(exports.memory.buffer) : null;
  window.Module.asm = exports;
  window.Module.wasmModule = module;
  window.Module.wasmInstance = instance;

  // 如果有 ___wasm_call_ctors，调用它来初始化 C++ 静态对象
  if (exports.___wasm_call_ctors) {
    exports.___wasm_call_ctors();
  }

  g_wasmModule = { instance, module, exports };
  console.log('WASM 模块加载完成');

  return g_wasmModule;
}

/**
 * 检查 WASM 是否已加载
 */
function isWasmLoaded() {
  return g_wasmLoaded;
}

/**
 * 获取 WASM 模块
 */
function getWasmModule() {
  return g_wasmModule;
}

// 导出全局函数和对象
window.WasmdDecrypt = WasmdDecrypt;
window.initPasswordPrompt = initPasswordPrompt;
window.loadEncryptedWasm = loadEncryptedWasm;
window.isWasmLoaded = isWasmLoaded;
window.getWasmModule = getWasmModule;
window.Module = window.Module || {};

// 页面加载时显示密码输入框
document.addEventListener('DOMContentLoaded', function() {
  // 如果没有指定回调，使用默认回调
  if (!g_onWasmReady) {
    g_onWasmReady = function() {
      console.log('WASM 已就绪，调用 initializeEncoder');
      if (typeof window.initializeEncoder === 'function') {
        window.initializeEncoder();
      }
    };
  }

  initPasswordPrompt();
});
