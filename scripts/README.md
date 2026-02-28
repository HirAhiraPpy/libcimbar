# Cimbar Web 编码器加密工具

本目录包含用于加密 Cimbar Web 编码器的 WASM 文件的工具脚本。

## 文件说明

- `encrypt-wasm.js` - 单独加密 WASM 文件的脚本
- `generate-encrypted-html.js` - 生成包含加密 WASM 的单文件 HTML

## 前置条件

1. Node.js 14+
2. 已构建的 WASM 文件 (`web/cimbar_js.wasm`)

## 使用方法

### 方法一：仅加密 WASM 文件

```bash
cd scripts/

# 使用随机口令加密
node encrypt-wasm.js

# 使用指定口令加密
node encrypt-wasm.js ../web/cimbar_js.wasm ../web/cimbar_js.wasm.enc "your-password"
```

输出：
- `cimbar_js.wasm.enc` - 加密的 WASM 文件（二进制格式）
- `cimbar_js.wasm.enc.b64` - Base64 编码版本

### 方法二：生成单文件 HTML

```bash
cd scripts/

# 使用随机口令生成
node generate-encrypted-html.js

# 使用指定口令生成
node generate-encrypted-html.js "your-password"
```

输出：
- `web/encoder-encrypted.html` - 包含加密 WASM 的 HTML 文件

## 部署步骤

1. **构建 WASM 文件**（如果还没有）:
   ```bash
   bash package-wasm.sh
   ```

2. **加密 WASM**:
   ```bash
   node scripts/encrypt-wasm.js ../web/cimbar_js.wasm ../web/cimbar_js.wasm.enc "your-password"
   ```

3. **部署文件到 Web 服务器**:
   - `web/index.html` - 主 HTML 文件
   - `web/main.js` - 编码器逻辑
   - `web/wasm-decrypt.js` - 解密模块
   - `web/cimbar_js.wasm.enc` - 加密的 WASM 文件
   - `web/cimbar_js.js` - WASM 加载器（如果需要）

4. **访问**:
   - 打开网页后会显示口令输入框
   - 输入正确的口令后加载编码器

## 安全说明

### 加密参数
- **密钥派生**: PBKDF2-HMAC-SHA256
- **迭代次数**: 100,000
- **加密算法**: AES-256-GCM
- **Salt**: 16 字节随机生成
- **IV**: 12 字节随机生成

### 口令建议
- 至少 12 个字符
- 包含大小写字母、数字、符号
- 避免使用常见单词或短语

### 限制
这是纯前端加密方案，主要目的是：
- 防止未经授权的随意使用
- 增加逆向工程难度
- 保护 WASM 代码知识产权

**注意**: 由于密钥最终需要在浏览器中使用，此方案无法完全防止有能力的攻击者。它主要用于访问控制，而非最高级别的安全保护。

## 文件结构

```
web/
├── index.html              # 主 HTML（深色霓虹主题）
├── main.js                 # 编码器逻辑
├── wasm-decrypt.js         # WASM 解密模块
├── cimbar_js.wasm.enc      # 加密的 WASM 文件
└── sw.js                   # Service Worker

scripts/
├── encrypt-wasm.js         # WASM 加密脚本
└── generate-encrypted-html.js  # 生成单文件 HTML
```

## 故障排除

### "无法加载加密 WASM 文件"
确保 `cimbar_js.wasm.enc` 文件存在于 web 目录中。

### "口令错误"
- 确认使用的口令与加密时设置的口令一致
- 检查加密文件是否损坏

### "WebAssembly.instantiate 失败"
- 检查浏览器是否支持 WebAssembly
- 确认解密后的数据是有效的 WASM 格式

## 视觉风格说明

编码器前端已更新为深色霓虹主题，与解码器保持一致：

- **背景**: #1a1a2e（深蓝色）
- **主色调**: #0ff（青色）、#0f0（绿色）
- **侧边栏**: 深色半透明背景
- **按钮**: 霓虹发光效果

## 常见问题

**Q: 为什么需要加密 WASM？**
A: 保护知识产权，防止未授权使用，增加逆向难度。

**Q: 加密会影响性能吗？**
A: 解密过程通常在 1-3 秒内完成（取决于文件大小和设备性能），之后编码器性能与原版相同。

**Q: 可以更换口令吗？**
A: 需要重新运行加密脚本生成新的加密文件。

**Q: 支持哪些浏览器？**
A: 支持所有现代浏览器（Chrome、Firefox、Safari、Edge），需要支持 WebAssembly 和 Web Crypto API。
