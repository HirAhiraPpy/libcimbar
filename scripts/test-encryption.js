#!/usr/bin/env node

/**
 * 测试加密/解密流程的脚本（不需要实际 WASM 文件）
 */

const crypto = require('crypto');

// 加密参数
const PBKDF2_ITERATIONS = 100000;
const KEY_LENGTH = 32;
const SALT_LENGTH = 16;
const IV_LENGTH = 12;

// 模拟 WASM 文件头（魔数）
const WASM_MAGIC = Buffer.from([0x00, 0x61, 0x73, 0x6d]); // "\0asm"

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

  return Buffer.concat([salt, iv, encrypted, authTag]);
}

function decrypt(encryptedBuffer, password) {
  const salt = encryptedBuffer.slice(0, SALT_LENGTH);
  const iv = encryptedBuffer.slice(SALT_LENGTH, SALT_LENGTH + IV_LENGTH);
  const encryptedDataWithTag = encryptedBuffer.slice(SALT_LENGTH + IV_LENGTH);

  const key = deriveKey(password, salt);
  const decipher = crypto.createDecipheriv('aes-256-gcm', key, iv);

  const encryptedData = encryptedDataWithTag.slice(0, -16);
  const authTag = encryptedDataWithTag.slice(-16);
  decipher.setAuthTag(authTag);

  let decrypted = Buffer.concat([
    decipher.update(encryptedData),
    decipher.final()
  ]);

  return decrypted;
}

function test() {
  console.log('=== 加密/解密流程测试 ===\n');

  // 创建测试数据（模拟 WASM 文件）
  const testData = Buffer.concat([
    WASM_MAGIC,
    Buffer.alloc(1000, 0x42) // 1KB 测试数据
  ]);
  console.log(`原始数据大小：${testData.length} bytes`);
  console.log(`WASM 魔数：${testData.slice(0, 4).toString('hex')}`);

  // 测试口令
  const password = 'test-password-123';

  // 加密
  console.log('\n--- 加密测试 ---');
  const encrypted = encrypt(testData, password);
  console.log(`加密后大小：${encrypted.length} bytes`);
  console.log(`加密数据（前 32 字节）: ${encrypted.slice(0, 32).toString('hex')}`);

  // 转换为 Base64
  const base64 = encrypted.toString('base64');
  console.log(`Base64 大小：${base64.length} characters`);

  // 解密（正确口令）
  console.log('\n--- 解密测试（正确口令）---');
  try {
    const decrypted = decrypt(encrypted, password);
    console.log(`解密后大小：${decrypted.length} bytes`);
    console.log(`WASM 魔数：${decrypted.slice(0, 4).toString('hex')}`);

    // 验证数据
    if (decrypted.equals(testData)) {
      console.log('✓ 解密成功，数据验证通过！');
    } else {
      console.log('✗ 解密失败，数据不匹配！');
    }
  } catch (err) {
    console.log('✗ 解密失败:', err.message);
  }

  // 解密（错误口令）
  console.log('\n--- 解密测试（错误口令）---');
  try {
    const decrypted = decrypt(encrypted, 'wrong-password');
    console.log('✗ 应该抛出错误！');
  } catch (err) {
    console.log('✓ 正确抛出错误:', err.message);
  }

  // Base64 编解码测试
  console.log('\n--- Base64 编解码测试 ---');
  const decoded = Buffer.from(base64, 'base64');
  if (decoded.equals(encrypted)) {
    console.log('✓ Base64 编解码正确');
  } else {
    console.log('✗ Base64 编解码失败');
  }

  console.log('\n=== 测试完成 ===\n');
}

test();
