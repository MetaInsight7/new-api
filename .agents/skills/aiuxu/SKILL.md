---
name: aiuxu
description: APIMart(aiuxu.com)视觉风格设计系统参考。当需要把 new-api web/classic 控制台/公开页对齐 APIMart 那种「Vercel/shadcn 极简」气质时使用(改配色、卡片、表格、按钮、徽章、侧栏、分页、筛选等呈现层)。包含从 APIMart 登录后台线上实测的 shadcn 主题 token、组件配方,以及到 new-api classic 主题层的落地映射。
metadata:
  author: project
  version: "1.0.0"
  source: "https://aiuxu.com/zh(APIMart 登录后台,getComputedStyle 实测,2026-07)"
---

# APIMart / aiuxu 设计系统(new-api classic 对齐参考)

把 `web/classic` 控制台(`.app-console` 作用域)对齐 APIMart(aiuxu.com)的视觉语言。**仅呈现层**:不改数据/路由/后端,不动受保护品牌(new-api / QuantumNous),默认只改亮色(`html:not(.dark)`)。

数据来源:APIMart 登录后台(总览 / 消费日志页)的 `getComputedStyle` 实测。APIMart 是 **Next.js + shadcn/ui("new-york" 风) + Tailwind + Geist 字体**,主题走标准 shadcn CSS 变量。

---

## 0. 核心气质(APIMart 的"味道")

1. **极简 · 扁平 · 无阴影**:卡片一律 `1px 边框 + 圆角`,**box-shadow: none**。靠浅边线 + 圆角 + 一层浅灰画布分层,不用投影。
2. **中性灰阶为主,绿色极克制**:整体是黑白灰(近黑文字 `#0a0a0a` + 多级灰),**唯一强调色是深绿 `#216d51`**,只用在:主按钮、激活分页、文字链接(详情)、正向徽章。绝不大面积铺绿。
3. **白卡衬浅灰画布**:页面/卡片/侧栏都是纯白 `#fff`,内容区画布用 `#fafafa` 把白卡衬出来 —— 层次靠 `#fff` vs `#fafafa` 的微差 + `#e5e5e5` 边框,而非颜色。
4. **紧凑**:控件小(按钮 sm 高 28px、字号 12–14px)、圆角中等(10px)、留白克制但呼吸感够。
5. **Geist 字体**:全站 Geist(sans)+ Geist Mono(数字/代码/密钥)。标题 weight 600,不用超粗。

> 与本仓库既有的 `dmit-design-system` 的区别:DMIT 偏「蓝 + 信号灯 + 进度条」的功能感;APIMart 更「黑白灰 + 一点深绿」的性冷淡极简感。二选一或按页面取舍。

---

## 1. 设计 Token(shadcn 变量,亮色实测)

| Token | 值 | 用途 |
|---|---|---|
| `--background` | `#ffffff` | 页面/卡片底 |
| `--foreground` | `#0a0a0a` | 主文字(近黑) |
| `--card` | `#ffffff` | 卡片背景 |
| `--primary` | **`#216d51`** | 主色(深绿)· 按钮/链接/激活 |
| `--primary-foreground` | `#ffffff` | 主色上的文字 |
| `--secondary` / `--muted` / `--accent` | `#f5f5f5` | 次级底 / 悬停底 / 选中底 |
| `--muted-foreground` | `#737373` | 次要/说明文字、表头标签 |
| `--secondary-foreground` | `#171717` | 次级控件文字 |
| `--destructive` | `#e7000b` | 危险/删除(红) |
| `--border` / `--input` | `#e5e5e5` | 边框 / 输入框边 |
| `--ring` | `#a1a1a1` | focus 环 |
| `--radius` | `0.625rem`(**10px**) | 圆角基准 |
| `--sidebar` | `#ffffff` | 侧栏底 |
| `--sidebar-accent` | `#f5f5f5` | 侧栏激活/悬停底 |
| `--sidebar-border` | `#e5e5e5` | 侧栏分隔 |
| 内容区画布(`main`) | **`#fafafa`** | 卡片背后的浅灰底(neutral-50) |
| chart 1→5 | `#d4d4d4 #737373 #525252 #404040 #262626` | 图表用灰阶(不用彩色) |

**圆角阶梯**(由 `--radius:10px` 派生,shadcn 惯例):
`sm = 6px` · `md = 8px` · `lg = 10px` · `xl = 14px`。卡片用 `lg(10px)`,按钮/输入/徽章用 `md(8px)`。

**字体**:
- Sans:`Geist, "Geist Fallback", ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`
- Mono:`Geist Mono`(数字/ID/密钥/时间戳)
- 正文 16px;标题 weight **600**(不超粗);说明文字用 `--muted-foreground` `#737373`。

---

## 2. 组件配方

### 2.1 页面外壳
- **顶栏**:白底 `bg-background/95` + `backdrop-blur`,`sticky top-0`。左 logo,中间导航(文字链接,`#737373`,悬停转近黑),右侧:额度胶囊(圆角 pill,显示余额)+ 主题/语言图标 + 头像。**底部 1px `#e5e5e5` 分隔**,无阴影。
- **侧栏**:白底,顶部可放分段切换(如 控制台/Agent)。菜单项:图标 + 文字,默认文字 `#737373`;**激活项** = `bg-sidebar-accent(#f5f5f5)` + 文字近黑,圆角 8px,无左侧强调条、无重色。底部放帮助链接 + 收起按钮。
- **内容区**:画布 `#fafafa`。页头 = 标题(weight 600)+ 一行灰色副标题(`#737373`),右侧放刷新/操作图标。下面是白卡。

### 2.2 卡片(扁平)
```
background: #fff;
border: 1px solid #e5e5e5;
border-radius: 10px;
box-shadow: none;          /* 关键:不投影 */
```
内边距常用 `16–24px`。卡片之间靠 `#fafafa` 画布 + 间距分隔。

### 2.3 按钮
- **主按钮**:`bg #216d51` / 文字 `#fff` / 圆角 8px / 无阴影。尺寸偏小:sm = 高 28px、padding `0 10px`、字号 12px、weight 500;默认 ≈ 高 36px。悬停轻微加深。
- **次按钮 / ghost**:透明或 `#f5f5f5` 底,文字近黑,`1px #e5e5e5` 边(outline 变体)。
- **危险**:`#e7000b`。

### 2.4 徽章 / 类型标签
小圆角 pill(`md`),浅底 + 同色系文字,字号 11–12px:
- 正向/充值:浅绿底 + 绿字(呼应主色系)。
- 中性/消费:浅灰/浅蓝底 + 灰字。
- 危险/失败:浅红底 + `#e7000b`。

### 2.5 数据表格(扁平,重点参考)
- 无外框重边;**表头**:`#737373` 小号标签,底部 1px 分隔线。
- 行:白底,行间 1px `#e5e5e5`(或更浅)分隔,行高统一;悬停行 `#fafafa`/`#f5f5f5`。
- **数字列右对齐**(Prompt/Completion/价格),用 Geist Mono、`tabular-nums`。
- 行内「详情」= 绿色文字链接(`#216d51`)。
- 密钥/ID/时间戳用 Mono。
- 空态:居中图标 + 灰字。

### 2.6 分页
底部右侧:上一页/下一页 + 页码。**当前页码 = `#216d51` 实心 + 白字**,其余页码为 ghost;右侧「跳转至 __ 页」+ 每页条数下拉。

### 2.7 筛选区
一行:搜索输入(带放大镜前缀)、日期选择、类型下拉(`所有类型`)。输入/下拉:`1px #e5e5e5` 边、圆角 8px、白底、focus 用 `--ring #a1a1a1`。

---

## 3. 落地到 new-api classic(`.app-console` 作用域)

- 只在 `html:not(.dark) .app-console` 下改,暗色不动。
- 若要整体切到这套:把内容区画布设 `#fafafa`、卡片改 `#fff + 1px #e5e5e5 + 10px 圆角 + 无阴影`、强调色统一为 `#216d51`(替换现有蓝 `#2563eb` 的语义位),字体接入 Geist。
- Semi 组件用作用域类覆盖(参照 `dmit-console.css` 的 `.app-console .semi-*` 做法);表格复用现有 `.dmit-flat-table` 扁平表模式(见 [[classic-flat-table-pattern]]),把强调色/边框/圆角换成本表值即可。
- 视口测试守住 `≤1280`(见 [[user-screen-1280]]),提交前先等用户检查(见 [[no-commit-until-user-checks]])。

## 4. 速用清单(一句话)
> 纯白卡 + `#fafafa` 画布 + `#e5e5e5` 1px 边 + 10px 圆角 + **无阴影**;黑白灰阶文字(`#0a0a0a`/`#737373`);唯一强调深绿 `#216d51`(按钮/链接/激活/正向徽章);Geist 字体、数字用 Geist Mono;控件紧凑(按钮 sm 28px);表格扁平、数字右对齐、详情走绿色链接。
