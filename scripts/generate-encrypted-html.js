#!/usr/bin/env node

/**
 * 生成加密 WASM 并嵌入 HTML 的脚本
 * 用于创建单文件部署的加密编码器
 *
 * 用法:
 *   node generate-encrypted-html.js [password]
 */

const fs = require('fs');
const crypto = require('crypto');
const path = require('path');

// 加密参数
const PBKDF2_ITERATIONS = 100000;
const KEY_LENGTH = 32; // 256 bits
const SALT_LENGTH = 16;
const IV_LENGTH = 12;

function generateSalt() {
  return crypto.randomBytes(SALT_LENGTH);
}

function generateIV() {
  return crypto.randomBytes(IV_LENGTH);
}

function deriveKey(password, salt) {
  return crypto.pbkdf2Sync(password, salt, PBKDF2_ITERATIONS, KEY_LENGTH, 'sha256');
}

function encrypt(buffer, password) {
  const salt = generateSalt();
  const iv = generateIV();
  const key = deriveKey(password, salt);

  const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);

  let encrypted = Buffer.concat([
    cipher.update(buffer),
    cipher.final()
  ]);

  const authTag = cipher.getAuthTag();

  // 输出格式：[salt:16][iv:12][encrypted_data:...][tag:16]
  const output = Buffer.concat([
    salt,
    iv,
    encrypted,
    authTag
  ]);

  return output;
}

function main() {
  const args = process.argv.slice(2);
  const password = args[0] || crypto.randomBytes(16).toString('hex');

  const wasmPath = path.join(__dirname, '..', 'web', 'cimbar_js.wasm');
  const encPath = path.join(__dirname, '..', 'web', 'cimbar_js.wasm.enc');
  const htmlPath = path.join(__dirname, '..', 'web', 'index.html');
  const outputPath = path.join(__dirname, '..', 'web', 'encoder-encrypted.html');

  console.log('=== 生成加密的 HTML 编码器 ===\n');

  // 检查 WASM 文件
  if (!fs.existsSync(wasmPath)) {
    console.error(`错误：找不到 WASM 文件 ${wasmPath}`);
    console.error('请先构建 WASM 文件：bash package-wasm.sh');
    process.exit(1);
  }

  // 读取 WASM 文件
  console.log('读取 WASM 文件...');
  const wasmBuffer = fs.readFileSync(wasmPath);
  console.log(`WASM 大小：${(wasmBuffer.length / 1024 / 1024).toFixed(2)} MB`);

  // 加密
  console.log('加密 WASM...');
  const encryptedBuffer = encrypt(wasmBuffer, password);
  console.log(`加密后大小：${(encryptedBuffer.length / 1024 / 1024).toFixed(2)} MB`);

  // 保存加密文件
  fs.writeFileSync(encPath, encryptedBuffer);
  console.log(`加密文件已保存：${encPath}`);

  // 转换为 Base64
  const base64Data = encryptedBuffer.toString('base64');
  console.log(`Base64 大小：${(Buffer.byteLength(base64Data) / 1024 / 1024).toFixed(2)} MB`);

  // 读取 HTML 模板
  console.log('读取 HTML 模板...');
  let htmlContent = fs.readFileSync(htmlPath, 'utf-8');

  // 将 Base64 数据嵌入 HTML
  const embedScript = `
  <script>
    // 加密的 WASM 数据（Base64 编码）
    window.ENCRYPTED_WASM_DATA = "${base64Data.substring(0, 100000)}";
    // 注意：由于 Base64 数据很长，实际部署时建议分割成多个变量或使用外部文件
  </script>
`;

  // 在 wasm-decrypt.js 之前插入嵌入脚本
  htmlContent = htmlContent.replace(
    '<script src="wasm-decrypt.js"></script>',
    embedScript + '\n  <script src="wasm-decrypt.js"></script>'
  );

  // 保存输出文件
  fs.writeFileSync(outputPath, htmlContent);
  console.log(`加密的 HTML 文件已保存：${outputPath}`);

  console.log('\n=== 生成完成 ===');
  console.log(`\n访问口令：${password}`);
  console.log('\n使用方法:');
  console.log(`1. 将 ${outputPath} 重命名为 index.html 或部署到 Web 服务器`);
  console.log(`2. 确保 cimbar_js.js 和 main.js 也在同一目录`);
  console.log(`3. 在浏览器中打开，输入口令即可使用`);
}

main();
