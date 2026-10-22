#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";

function usage() {
  console.log(`Usage:
  node scripts/init-clone.mjs <slug> [--url <url>] [--mode <mode>] [--level <L1-L6>] [--root <dir>] [--in-place]

Creates the project directory and RECON/screenshots/. Creates NOTES.md only if
it does not already exist and ensures the required entries are in .gitignore.

With --in-place, existing NOTES.md is preserved and missing ignore entries are
appended without replacing existing .gitignore rules.
`);
}

function parseArgs(argv) {
  const out = { slug: null, url: "", mode: "", level: "", root: process.cwd(), inPlace: false };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    if (arg === "--help" || arg === "-h") out.help = true;
    else if (arg === "--url") out.url = argv[++i] || "";
    else if (arg === "--mode") out.mode = argv[++i] || "";
    else if (arg === "--level") out.level = argv[++i] || "";
    else if (arg === "--root") out.root = argv[++i] || process.cwd();
    else if (arg === "--in-place") out.inPlace = true;
    else if (!out.slug) out.slug = arg;
    else throw new Error(`Unexpected argument: ${arg}`);
  }
  return out;
}

function cleanSlug(input) {
  return input
    .trim()
    .toLowerCase()
    .replace(/https?:\/\//g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function ensureIgnoreEntries(file, required) {
  let stats;
  try {
    stats = fs.lstatSync(file);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
  if (stats?.isSymbolicLink()) {
    throw new Error(`Refusing to update a symbolic-link .gitignore: ${file}`);
  }
  if (stats && !stats.isFile()) {
    throw new Error(`Refusing to update a non-file .gitignore: ${file}`);
  }
  const content = stats ? fs.readFileSync(file, "utf8") : "";
  const normalize = (rule) =>
    rule
      .trim()
      .replace(/^!/, "")
      .replace(/^\/+/, "")
      .replace(/\/+$/, "")
      .replace(/^\*\*\//, "");
  const lines = content.split(/\r?\n/);
  const missing = required.filter((entry) => {
    let ignored = false;
    for (const line of lines) {
      const rule = line.trim();
      if (!rule || rule.startsWith("#")) continue;
      const normalized = normalize(rule);
      if (normalized === entry) ignored = !rule.startsWith("!");
    }
    return !ignored;
  });
  if (missing.length === 0) return;

  const newline = content.includes("\r\n") ? "\r\n" : "\n";
  const separator = content && !content.endsWith("\n") ? newline : "";
  fs.appendFileSync(file, `${separator}${missing.join(newline)}${newline}`, {
    flag: stats ? "a" : "ax",
  });
}

function writeNotesIfMissing(file, contents) {
  if (fs.existsSync(file)) {
    const stats = fs.lstatSync(file);
    if (!stats.isFile() && !stats.isSymbolicLink()) {
      throw new Error(`Refusing to replace a non-file NOTES.md: ${file}`);
    }
    return;
  }
  fs.writeFileSync(file, contents, { flag: "wx" });
}


function notesTemplate({ name, url, mode, level }) {
  return `# ${name} · 克隆笔记

## 源信息
- 原站 URL: ${url}
- 源码仓库:
- 原作者:
- 许可证:
- 致谢要求:

## 技术栈
- 框架 / 关键库 / Node 版本:

## 复刻前预判
- 复杂度等级: ${level}
- 推荐模式: ${mode}
- 可高保真的部分:
- 需要近似或替代的部分:
- 不克隆的部分:
- 主要风险:

## 跑起来
\`\`\`bash
python3 -m http.server 8123
\`\`\`

## 改了什么（对照原版）
-

## 原站 vs 克隆站
| 模块 | 原站表现 | 克隆实现 | 差异 / 取舍 | 证据 |
|---|---|---|---|---|
| 首屏 |  |  |  |  |
| 导航 |  |  |  |  |
| 核心动效 |  |  |  |  |
| 内容区块 |  |  |  |  |
| 移动端 |  |  |  |  |

## 复刻评分
- 源证据: /5
- 结构保真: /5
- 视觉保真: /5
- 动效/交互: /5
- 响应式: /5
- 功能完整: /5
- 内容替换: /5
- 法务/部署风险: /5
- 总评:

## 替换地图（要换什么改哪）
- 文字 -> 文件 行
- 图片/媒体 -> 目录
- 配色 -> CSS 变量 / theme
- 3D 模型 / 字体 ->

## 验证
- [ ] 本地跑通、console 0 error
- [ ] 截图对照原站（RECON/screenshots/）
- 验证不了的点（如实记，别伪造）:
`;
}

try {
  const args = parseArgs(process.argv.slice(2));
  if (args.help || !args.slug) {
    usage();
    process.exit(args.help ? 0 : 1);
  }

  const slug = cleanSlug(args.slug);
  if (!slug) throw new Error("Slug is empty after normalization.");
  const name = slug.endsWith("-clone") ? slug : `${slug}-clone`;
  const root = path.resolve(args.root || process.cwd());
  const project = args.inPlace ? root : path.join(root, name);

  if (!args.inPlace && fs.existsSync(project)) {
    throw new Error(`Project already exists: ${project}`);
  }

  const notesPath = path.join(project, "NOTES.md");
  const ignorePath = path.join(project, ".gitignore");
  fs.mkdirSync(path.join(project, "RECON", "screenshots"), { recursive: true });
  writeNotesIfMissing(
    notesPath,
    notesTemplate({
      name,
      url: args.url,
      mode: args.mode,
      level: args.level,
    })
  );
  ensureIgnoreEntries(ignorePath, ["node_modules", ".DS_Store"]);

  console.log(project);
} catch (error) {
  console.error(`init-clone failed: ${error.message}`);
  process.exit(1);
}
