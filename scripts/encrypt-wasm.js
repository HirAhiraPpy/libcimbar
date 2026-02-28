#!/usr/bin/env node

/**
 * WASM 加密脚本
 * 使用 AES-256-GCM 加密 cimbar_js.wasm 文件
 *
 * 用法:
 *   node encrypt-wasm.js [input] [output] [password]
 *
 * 默认:
 *   input: web/cimbar_js.wasm
 *   output: web/cimbar_js.wasm.enc
 *   password: 随机生成或使用提供的口令
 */

const fs = require('fs');
const crypto = require('crypto');

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

  // 获取认证标签
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

  const inputFile = args[0] || 'web/cimbar_js.wasm';
  const outputFile = args[1] || 'web/cimbar_js.wasm.enc';
  let password = args[2];

  console.log('=== Cimbar WASM 加密工具 ===\n');
  console.log(`输入文件：${inputFile}`);
  console.log(`输出文件：${outputFile}`);

  // 检查输入文件
  if (!fs.existsSync(inputFile)) {
    console.error(`错误：找不到输入文件 ${inputFile}`);
    process.exit(1);
  }

  // 如果没有提供口令，生成随机口令
  if (!password) {
    password = crypto.randomBytes(16).toString('hex');
    console.log(`\n⚠️  使用随机生成的口令：${password}`);
    console.log('请妥善保管此口令！\n');
  } else {
    console.log(`使用提供的口令进行加密\n`);
  }

  // 读取 WASM 文件
  console.log('读取 WASM 文件...');
  const wasmBuffer = fs.readFileSync(inputFile);
  console.log(`WASM 文件大小：${(wasmBuffer.length / 1024).toFixed(2)} KB`);

  // 加密
  console.log('正在加密...');
  const startTime = Date.now();
  const encryptedBuffer = encrypt(wasmBuffer, password);
  const encryptTime = Date.now() - startTime;
  console.log(`加密完成，耗时：${encryptTime}ms`);
  console.log(`加密后大小：${(encryptedBuffer.length / 1024).toFixed(2)} KB`);

  // 写入输出文件
  console.log(`写入文件：${outputFile}`);
  fs.writeFileSync(outputFile, encryptedBuffer);

  // 生成 Base64 编码版本（用于嵌入 HTML）
  const base64Output = outputFile + '.b64';
  const base64Data = encryptedBuffer.toString('base64');
  fs.writeFileSync(base64Output, base64Data);
  console.log(`Base64 编码版本：${base64Output}`);
  console.log(`Base64 大小：${(Buffer.byteLength(base64Data) / 1024).toFixed(2)} KB`);

  console.log('\n=== 加密完成 ===');
  console.log(`\n口令：${password}`);
  console.log('\n提示：将加密文件部署到 web 目录，并在前端使用相同的口令进行解密。');
}

main();
