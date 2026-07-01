# HeroUI Pro 全家桶配置指南（给另一个 Agent）

把这份发给另一个 Agent，让它照着做就能配齐 HeroUI Pro 的 MCP + Skills。

## 目标

给项目装上 HeroUI Pro 全家桶：
- **2 个 MCP server**（react + native，实时拉组件 docs / CSS / 源码 / 主题）
- **3 个 Pro skill**（react-pro / native-pro / design-taste，知识层）
- **3 个 OSS skill**（react / native / migration，已随 hp-skills 一起）

最终 Agent 写 HeroUI 代码时，既能拿到知识层（skill 教怎么写），又能拿到实时数据层（MCP 拉最新组件 API）。

---

## 前置：认证体系（关键，别搞混）

HeroUI 有 **两套不同的认证**，必须分开用，不能混：

| 认证 | 形态 | 用途 | 环境变量 |
|------|------|------|----------|
| **Personal Token** | UUID 形如 `0596988c-4375-41e0-98b7-61f22bddf6f1` | MCP server 鉴权 + Skills tar.gz 拉取 | `HEROUI_PERSONAL_TOKEN` |
| **HP Key** | 以 `hp_` 开头，如 `hp_a22d5ab29cff21b099552f1b` | `hpsetup` 装 `@heroui-pro/react` npm 包本身 | `HEROUI_KEY` |

**本指南只配置 MCP + Skills，全程用 Personal Token，不碰 HP Key。**
HP Key 那条是给 `hpsetup` 装 npm 包用的，与本指南无关。

Personal Token 从 https://heroui.pro/dashboard 获取。

---

## Step 1：写 `.mcp.json`（项目根）

在项目根创建 `.mcp.json`，配 2 个 HeroUI Pro MCP server + 可选的 context7/playwright：

```json
{
  "mcpServers": {
    "heroui-pro": {
      "type": "stdio",
      "command": "npx",
      "args": [
        "-y",
        "hpmcp@latest",
        "react",
        "0596988c-4375-41e0-98b7-61f22bddf6f1"
      ]
    },
    "heroui-native-pro": {
      "type": "stdio",
      "command": "npx",
      "args": [
        "-y",
        "hpmcp@latest",
        "native",
        "0596988c-4375-41e0-98b7-61f22bddf6f1"
      ]
    }
  }
}
```

**关键点（这些是我踩过的坑，别重蹈）：**

1. **MCP server 是 `hpmcp` npm 包，stdio transport**——不是远程 HTTP server。
   - ❌ 不要用 `https://mcp.heroui.pro/mcp`（远程 HTTP 端点）——用个人 token 走这条路会返回 401「Invalid or expired personal token」，**token 本身没问题，是端点走错了**。
   - ❌ 不要用 `@heroui-pro/mcp` 这个 npm 包名——404 不存在。
   - ❌ 不要用 `heroui-pro` CLI 的 `mcp` 子命令——这个 CLI（v1.0.0-beta.12）没有 mcp 子命令，只有 install/update/react 等。
   - ✅ 正确：`npx -y hpmcp@latest <react|native> <PERSONAL_TOKEN>`，本地起 stdio server。

2. **token 作为 args 末尾位置参数传，不是 env**。`hpmcp` 的调用约定是 `hpmcp <platform> <token>`，token 进 args，不进 env。

3. **react 和 native 是两个独立 server**，要分别配（args 第二个参数 `react` vs `native`）。

**实测结果（我跑通的）：**
- `heroui-pro` (react) → serverInfo `@heroui-pro/react-mcp@0.2.0`，8 个工具：
  `list_components` / `get_component_docs` / `get_chat_export_manifest` / `get_chat_export_files` / `get_component_source_code` / `get_css` / `get_docs` / `get_theme_variables`
- `heroui-native-pro` (native) → serverInfo `@heroui-pro/native-mcp@0.2.0`，4 个工具：
  `list_components` / `get_component_docs` / `get_docs` / `get_theme_variables`
  （native 少 4 个，因为 Uniwind 处理样式不需要 get_css、native 源码不开放、无 chat export）

---

## Step 2：安装 3 个 Pro Skill

Pro skill 是从 `hp-skills.932324.xyz` 拉的 tar.gz，用 Personal Token 鉴权，解压到项目的 `.atomcode/skills/`（或你 Agent framework 对应的 skills 目录）。

**3 个 skill：**
| skill | 内容 |
|-------|------|
| `heroui-react-pro` | 55 个 react Pro 组件 + v3 compound API 规则 + design tokens |
| `heroui-native-pro` | 30 个 native Pro 组件（日期选择器、步进器、进度按钮等） |
| `heroui-pro-design-taste` | 78 条设计原则 / 10 类（spacing/typography/color/cards/forms/buttons/icons/navigation/a11y） |

**安装方式 A：bash + curl（官方一行命令，Linux/macOS 或 Windows Git Bash）**

```bash
curl -fsSL https://hp-skills.932324.xyz/install | \
  HEROUI_PERSONAL_TOKEN=0596988c-4375-41e0-98b7-61f22bddf6f1 bash -s heroui-react-pro

curl -fsSL https://hp-skills.932324.xyz/install | \
  HEROUI_PERSONAL_TOKEN=0596988c-4375-41e0-98b7-61f22bddf6f1 bash -s heroui-native-pro

curl -fsSL https://hp-skills.932324.xyz/install | \
  HEROUI_PERSONAL_TOKEN=0596988c-4375-41e0-98b7-61f22bddf6f1 bash -s heroui-pro-design-taste
```

官方脚本会自动检测 `~/.claude/skills/`、`~/.cursor/skills/` 等目录并装进去。**但如果你用的是别的 Agent framework（如 AtomCode），官方脚本检测不到，需要用方式 B。**

**安装方式 B：自定义目录（给 AtomCode 等非主流 Agent）**

官方脚本支持 `HEROUI_PRO_SKILLS_DIR` 环境变量指定目录：

```bash
HEROUI_PRO_SKILLS_DIR=.atomcode curl -fsSL https://hp-skills.932324.xyz/install | \
  HEROUI_PERSONAL_TOKEN=0596988c-4375-41e0-98b7-61f22bddf6f1 bash -s heroui-react-pro
```

（脚本会在 `$HEROUI_PRO_SKILLS_DIR/skills/<name>/` 下解压）

**安装方式 C：Windows cmd.exe 无 bash 时，用 Node.js 脚本**

把下面这个脚本存成 `.atomcode/scripts/install-heroui-pro.cjs`，跑 `set HEROUI_PERSONAL_TOKEN=<token>&& node .atomcode/scripts/install-heroui-pro.cjs`：

```javascript
// HeroUI Pro skill installer
// Usage: set HEROUI_PERSONAL_TOKEN=<token>&& node install-heroui-pro.cjs
const https = require('https');
const fs = require('fs');
const path = require('path');
const zlib = require('zlib');

const TOKEN = process.env.HEROUI_PERSONAL_TOKEN;
if (!TOKEN) { console.error('HEROUI_PERSONAL_TOKEN env required'); process.exit(1); }

const SKILLS = ['heroui-react-pro', 'heroui-pro-design-taste', 'heroui-native-pro'];
const DEST = path.resolve(__dirname, '..', 'skills'); // 调整到你的 skills 目录

function fetchTar(url) {
  return new Promise((resolve, reject) => {
    const req = https.request(url, { headers: { 'x-heroui-personal-token': TOKEN } }, res => {
      if (res.statusCode !== 200) { res.destroy(); reject(new Error('HTTP ' + res.statusCode)); return; }
      resolve(res);
    });
    req.on('error', reject); req.end();
  });
}

function extractTarGz(stream, destDir) {
  return new Promise((resolve, reject) => {
    const gunzip = zlib.createGunzip();
    stream.pipe(gunzip);
    gunzip.on('error', reject);
    let buf = Buffer.alloc(0);
    gunzip.on('data', c => { buf = Buffer.concat([buf, c]); });
    gunzip.on('end', () => {
      try {
        let off = 0; const files = [];
        while (off < buf.length) {
          const header = buf.subarray(off, off + 512);
          if (header.length < 512 || header.every(b => b === 0)) break;
          const name = header.subarray(0, 100).toString('utf8').replace(/\0+$/, '');
          if (!name) break;
          const size = parseInt(header.subarray(124, 136).toString('utf8').replace(/\0+$/, '').trim() || '0', 8);
          const typeflag = String.fromCharCode(header[156] || 0x30);
          off += 512;
          const content = buf.subarray(off, off + size);
          if (typeflag === '0' || typeflag === '' || typeflag === '\x00') {
            const fp = path.join(destDir, name);
            fs.mkdirSync(path.dirname(fp), { recursive: true });
            fs.writeFileSync(fp, content);
            files.push(name);
          } else if (typeflag === '5') {
            fs.mkdirSync(path.join(destDir, name), { recursive: true });
          }
          off += Math.ceil(size / 512) * 512;
        }
        resolve(files);
      } catch (e) { reject(e); }
    });
  });
}

(async () => {
  fs.mkdirSync(DEST, { recursive: true });
  for (const skill of SKILLS) {
    console.log('=== ' + skill + ' ===');
    try {
      const stream = await fetchTar('https://hp-skills.932324.xyz/skills/' + skill + '.tar.gz');
      const skillDir = path.join(DEST, skill);
      fs.mkdirSync(skillDir, { recursive: true });
      await extractTarGz(stream, skillDir);
      console.log('  OK: ' + (fs.existsSync(path.join(skillDir, 'SKILL.md')) ? 'SKILL.md present' : 'WARN no SKILL.md'));
    } catch (e) { console.error('  FAIL: ' + e.message); process.exitCode = 1; }
  }
})();
```

---

## Step 3（可选）：补 3 个 OSS skill

OSS skill（`heroui-react` / `heroui-native` / `heroui-migration`）不在 hp-skills CDN 上，是从 HeroUI 官方别的渠道拿的。如果你项目里已经有了（比如在 `.agents/skills/`），直接复制到你的 skills 目录即可。如果没有，可省略——Pro skill 已经覆盖了 Pro + OSS 两套的用法知识。

---

## Step 4：验证

**验证 MCP 连通**（最关键，确保 token + 包都可用）。存成脚本跑：

```javascript
// test-hpmcp-stdio.cjs  —  node test-hpmcp-stdio.cjs react  或  native
const { spawn } = require('child_process');
const TOKEN = '0596988c-4375-41e0-98b7-61f22bddf6f1';
const PLATFORM = process.argv[2] || 'react';
const child = spawn('npx', ['-y', 'hpmcp@latest', PLATFORM, TOKEN], { stdio: ['pipe','pipe','pipe'], shell: process.platform === 'win32' });
let pending = '', initialized = false;
const timer = setTimeout(() => { console.log('TIMEOUT'); child.kill(); }, 10000);
child.stdout.on('data', chunk => {
  pending += chunk.toString();
  let idx;
  while ((idx = pending.indexOf('\n')) >= 0) {
    const line = pending.slice(0, idx).trim(); pending = pending.slice(idx+1);
    if (!line) continue;
    let p; try { p = JSON.parse(line); } catch (e) { continue; }
    if (!initialized && p.id === 1 && p.result) {
      initialized = true;
      console.log('init OK: ' + JSON.stringify(p.result.serverInfo));
      child.stdin.write(JSON.stringify({ jsonrpc:'2.0', method:'notifications/initialized' }) + '\n');
      child.stdin.write(JSON.stringify({ jsonrpc:'2.0', id:2, method:'tools/list', params:{} }) + '\n');
    } else if (initialized && p.id === 2 && p.result) {
      console.log('tools: ' + p.result.tools.map(t => t.name).join(', '));
      clearTimeout(timer); child.kill(); process.exit(0);
    }
  }
});
setTimeout(() => child.stdin.write(JSON.stringify({ jsonrpc:'2.0', id:1, method:'initialize', params:{ protocolVersion:'2025-06-18', capabilities:{}, clientInfo:{ name:'verify', version:'1' } } }) + '\n'), 3000);
```

**期望输出：**
- react → `init OK: {"name":"@heroui-pro/react-mcp","version":"0.2.0"}` + 8 个工具名
- native → `init OK: {"name":"@heroui-pro/native-mcp","version":"0.2.0"}` + 4 个工具名

**如果 401「Invalid or expired personal token」** → 检查是不是走错了端点（远程 HTTP 会 401，stdio 不会）；如果 stdio 也 401，才是 token 真过期，去 https://heroui.pro/dashboard 换新的。

---

## 最终目录结构

```
项目根/
├── .mcp.json                          ← 2 个 heroui-pro MCP server（stdio + hpmcp）
└── .atomcode/skills/                  （或你的 Agent 的 skills 目录）
    ├── heroui-react-pro/SKILL.md      ← 55 个 react Pro 组件
    ├── heroui-native-pro/SKILL.md     ← 30 个 native Pro 组件
    └─ heroui-pro-design-taste/SKILL.md ← 78 条设计原则
```

---

## Token 更新流程（token 过期时）

1. 去 https://heroui.pro/dashboard 拿新 Personal Token
2. 改 `.mcp.json` 里 2 个 server args 末尾的 token
3. 改 install 脚本里的 `TOKEN`（或重新 `set HEROUI_PERSONAL_TOKEN=<新>`）
4. 重跑 `test-hpmcp-stdio.cjs react` 和 `native` 确认通

---

## 踩坑总结（给接手 Agent 的忠告）

1. **不要猜 MCP server 的实现形式**——先查官方文档 `https://heroui.pro/docs/react/getting-started/mcp-server`，确认是 stdio 还是 HTTP、包名是什么。
2. **Personal Token 和 HP Key 不混用**——MCP/Skills 用 Personal Token，hpsetup 装 npm 包用 HP Key。
3. **远程 HTTP `mcp.heroui.pro` 不是给个人 token 用的**——那条路会 401，正确做法是 `hpmcp` npm 包本地 stdio。
4. **react 和 native 是两个独立 server**，要分别配、分别测。
5. **Pro skill 是离线知识层**，MCP 是在线数据层，两者互补不互替——都要装。
6. **token 在 args 里位置固定**：`hpmcp <platform> <token>`，token 是最后一个位置参数。
