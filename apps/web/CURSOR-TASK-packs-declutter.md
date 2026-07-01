# 施工单：packs 页「看得见的减噪」（交给 Cursor 执行）

> 这份文件是给执行 AI（Cursor）的**完整、自包含**任务说明。你**没有**之前对话的上下文，所有约束都写在下面，照做即可。做完我（架构 reviewer）会逐行 review diff。

## 0. 背景一句话

`apps/web/src/app/packs/page.tsx`（2940 行）是云雀 Agent 信息密度最高、视觉最"花/挤"的页面。
前几轮已经做完**内部规范化**（颜色换语义 token、Chip 语义化、过滤器换 Pro Segment）——但这些**肉眼几乎看不出变化**。
**这一轮要做的是用户一眼能看出的「减法」**：降低每张能力包卡片的视觉密度、加大留白、折叠次要信息。

**核心原则（HeroUI Pro 设计纪律）**：
- 同一份数据只用**一种最有效**的表达，删掉冗余。
- **减边框**：只保留有结构意义的边框，去掉装饰性/习惯性边框。
- 中性面、语义色只表真实语义（成功/警告/危险/强调），不当装饰。
- 留白要慷慨（`gap-4`/`gap-6`/`p-5`），不要塞满。
- 不要玻璃叠玻璃（半透明面里再套半透明面）。

---

## 1. 绝对不能碰的东西（碰了测试就红 / 功能就坏）

测试文件：`apps/web/src/app/__tests__/packs-page.test.tsx`（**必须保持 11/11 通过**）。它断言以下内容，**改视觉时必须原样保留**：

1. **中文文本原样**：`验源`、`运行`、`信任`、`交付`、`有后端能力`、`安装前只读检查`、`说明完整`、`需留意`、`低风险`、`测试版`、`已安装 · 已启用`、`交付状态：…`、`如何使用`、`安装前看这几点`、各卡片标题等——**一个字都不能改**。
2. **`.section-card` class**：每张卡片外层 `<Card className="section-card pack-card …">` 的 `section-card` 和 `pack-card` 两个 class **必须保留**（测试用它定位卡片）。
3. **`aria-expanded` 展开门控**：`expandedInstallableCards` / `togglePackDetails` 相关的 `aria-expanded={…}` 和 `hidden={…}` 逻辑**必须保留**，展开/收起文案（`展开详情`/`收起详情`/`收起`等）保留。
4. **`advancedVisible` 门控**：所有 `advancedVisible && (…)` 和 `advancedVisible ? … : …` 的可见性条件**必须保留**（普通模式 vs 维护模式显示不同内容，测试两种都查）。
5. **链接 href**：`packStudioHref(…)`、`/packs/detail?id=…`、`/packs/…` 等所有 `href` **必须保留**。
6. **按钮 role/name**：所有 `<Button>`、`<Link>` 的可见文字保留（测试用 `getByRole("button"/"link", { name })` 找）。
7. **过滤器是 Pro `Segment`**：`renderFilterGroup` 已用 `<Segment>`/`<Segment.Item>`，**别动它**（测试用 `getByRole("radio", …)` 点选项）。

> 改动前先跑一次基线确认绿：
> ```
> cd apps/web && npx vitest run src/app/__tests__/packs-page.test.tsx
> ```
> 应为 `Tests 11 passed`。

---

## 2. 要做的减法（按优先级，每步独立、改完即可验证）

### ★ 任务 A：把 `PackTrustStrip` 从「4 个带边框小盒子」改成「一行内联事实」（最高收益）

**位置**：`PackTrustStrip` 组件，定义在**第 233 行**。

**现状**：它把 `验源 / 运行 / 信任 / 交付` 渲染成一个 **2×2 或 1×3 的网格，每格是一个 `rounded-lg border px-2.5 py-2` 的盒子**（见第 285-302 行）。每张卡片因此多出 4 个带边框的小面板——这是全页最大的视觉噪音源。

**改法（减法）**：
- 删掉每个 fact 外层的 `rounded-lg border px-2.5 py-2` 盒子和网格容器。
- 改成**一行（窄屏可换行）的内联事实**：每个 fact = 一个小图标 + label + value，用 `gap` 分隔，**不要边框、不要背景块**。
- 视觉参考（结构，不是字面代码）：
  ```
  <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs" aria-label="能力包状态摘要">
    {facts.map((fact) => (
      <span key={fact.key} className="inline-flex items-center gap-1.5 min-w-0">
        <span style={{ color: trustToneStyle(fact.tone).color }}>{fact.icon}</span>
        <span style={{ color: "var(--yunque-text-muted)" }}>{fact.label}</span>
        <span className="truncate" title={fact.value} style={{ color: "var(--yunque-text)" }}>{fact.value}</span>
      </span>
    ))}
  </div>
  ```
- **保留**：`aria-label="能力包状态摘要"`、每个 fact 的 `label`/`value`/`icon`/`tone`、`facts` 数组的构造逻辑（第 254-275 行）**完全不动**，只改最后的 `return (…)` 渲染部分（第 280 行起）。
- `fact.hint` 仍然渲染（如果有），但跟在 value 后面或下方一行小字即可，不要再套盒子。
- 语义色（`trustToneStyle(fact.tone).color`）只用在**图标**上，label 用 `--yunque-text-muted`，value 用 `--yunque-text`——颜色只点缀图标，不刷整块。

**验证**：`验源/运行/信任/交付` 四个词和它们的 value 仍在 DOM 里（测试查 `验源`/`信任`/`交付`/`运行`）。跑测试应仍 11/11。

---

### 任务 B：能力包图标去掉彩色渐变方块

**位置**：第 **2537-2545 行**（`renderInstallableCard` 内）和第 **2746 行附近**（`renderPackCard` 内，结构相同）。

**现状**：图标外面套了一个 `linear-gradient(135deg, rgba(59,130,246,0.1) 0%, rgba(147,51,234,0.1) 100%)` 蓝紫渐变 + `rgba(255,255,255,0.05)` 边框的方块。这是写死的、跟主题不搭的装饰色。

**改法**：把那个渐变方块的 `style` 改成中性：
```
style={{ background: "var(--yunque-bg-muted)", border: "1px solid var(--glass-edge)" }}
```
两处都改（2540-2541 和 2746-2747）。图标本身 `<PackageCheck … style={{ color: "var(--yunque-accent)" }}>` 保留——让 accent 色只落在图标上。

**验证**：全页 `grep -n "linear-gradient.*rgba" src/app/packs/page.tsx` 应为空。

---

### 任务 C：展开详情里「4 连面板」瘦身（中收益）

**位置**：`renderInstallableCard` 的展开区，第 **2624-2690 行**（`renderPackCard` 里有结构相同的一段，一起改）。

**现状**：展开后依次是 4 个 `rounded-xl … p-4` 的面板（来源信息 / 交付状态 / 如何使用 / 安装前看这几点），层层叠叠。

**改法（克制，别改文案和逻辑）**：
- 把这几个面板的 `p-4` 统一降到 `p-3`，面板之间的 `space-y-4` 降到 `space-y-3`，让密度均匀、留白一致。
- 「来源信息」那块（2625-2633，纯 `key: value` 列表）**去掉外层 `rounded-xl border … bg` 盒子**，直接作为缩进小字列表呈现（它只是元数据，不需要面板包裹）。
- 其余 3 个面板**保留**（交付状态/如何使用/安装前看这几点有结构意义），只统一 padding。
- **所有文案、`Chip`、`Link`、href、map 逻辑原样保留。**

**验证**：`交付状态：`、`如何使用`、`安装前看这几点` 仍在；测试 11/11。

---

### 任务 D（可选，做完 A-C 且测试绿再做）：去重两个卡片渲染函数

**位置**：`renderInstallableCard`（第 2495 行）和 `renderPackCard`（第 2697 行）——约 440 行高度重复。

**改法**：抽公共结构到一个 `PackCardShell` 组件/函数（卡片外壳 + header + trust strip + footer），两个函数复用。
**风险高**：这两个函数闭包了约 10 个组件内 state（`busy`、`advancedVisible`、`expandedInstallableCards`、`expandedPackCards`、各种 toggle 函数等）。抽取时**必须把这些当 props 显式传进去**，不能漏。

> **如果对 state 穿参没把握，这一步先跳过**，只交 A/B/C。A/B/C 已经能带来肉眼可见的减噪。D 留给 reviewer 决定。

---

## 3. 完成后必须自检（按顺序，全绿才算完成）

```bash
cd apps/web

# 1. 类型检查：packs 页必须零错误
npx tsc --noEmit -p tsconfig.json 2>&1 | grep "app/packs/page"
#   ↑ 输出为空 = 通过

# 2. packs 页测试：必须 11 passed
npx vitest run src/app/__tests__/packs-page.test.tsx
#   ↑ "Tests 11 passed" = 通过

# 3. 确认渐变已清除（任务 B）
grep -n "linear-gradient.*rgba" src/app/packs/page.tsx
#   ↑ 输出为空 = 通过
```

**只要这三项有任何一项不绿，就是没做完，继续修，不要交。**

---

## 4. 交付物

- 改完的 `apps/web/src/app/packs/page.tsx`
- 一句话说明你做了 A/B/C 里的哪几项、D 做没做（没做说明原因）
- 三项自检的实际输出（贴出来）

> reviewer 会重点看：测试是否真绿、`.section-card`/`aria-expanded`/`advancedVisible`/href 是否原样、有没有偷偷改中文文案、减边框是否只减了装饰性的。
