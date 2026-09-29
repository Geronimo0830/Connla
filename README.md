# Connla

Connla 是一个以本地使用为主的个人知识库，基于 [Memos v0.31.0](https://github.com/usememos/memos) 开发。它保留了快速记录的基础能力，并增加文档收件箱、主题、知识卡片、全文搜索，以及本地备份与恢复。

> 这是源码仓库，不包含任何用户数据、备份或预编译的 Windows 程序。当前主要使用场景是单用户、本机 SQLite。请自行保管账号密码，并定期将备份复制到其他磁盘。

## 已实现的内容

- 记录想法，上传并保存原始文档；解析出的文字作为可重新生成的派生内容，不改写原件。
- 在文档收件箱中阅读与管理常见格式，包括 TXT、Markdown、PDF、DOCX 和 XLSX。
- 按关键词、内容类型、文件格式和日期筛选搜索结果。
- 整理主题、编写知识卡片，并在本地备份与恢复。
- 在 Windows 上从源码打包本地版；运行时仅监听 `127.0.0.1:8081`。

旧版 `.doc` / `.xls` 不做文字识别；扫描版 PDF 不提供 OCR。AI 自动生成和复习功能不在当前版本范围内。若导入格式不受支持，原文件仍应保留，界面会显示处理状态。

## 从源码运行

开发环境需要 Go 1.27、Node.js 24 和 pnpm 11。仓库保留了 Memos 原有的 Go module 路径及部分内部命名，以兼容现有代码和生成文件；这不影响产品名称 Connla。

```powershell
cd web
pnpm install --frozen-lockfile
pnpm release
cd ..
go run ./cmd/memos --port 8081
```

在浏览器访问 `http://127.0.0.1:8081/`。第一次使用时创建管理员账号。Windows 本地版的构建、启动和数据位置见 [使用说明](docs/LOCAL_WINDOWS.md)；备份和恢复见 [备份说明](docs/BACKUP_RESTORE.md)。打包脚本会把构建结果放入被 Git 忽略的 `build/`，不要把该目录或个人数据目录提交到公开仓库。

## 开发检查

```powershell
go test ./...
cd web
pnpm lint
pnpm test
pnpm build
```

部分 Go 存储测试需要 Docker。协议文件的检查命令是 `cd proto; buf lint`。如果修改 `.proto`，请按仓库原有流程重新生成 Go、TypeScript 和 OpenAPI 文件。

## 来源与许可

Connla 基于 MIT 许可的 [Memos](https://github.com/usememos/memos)；保留了其 [MIT 许可证与署名](LICENSE)。其他已引入组件见 [第三方声明](THIRD_PARTY_NOTICES.md)。

软件源码按根目录的 MIT 许可证开放，但 Connla 名称和图标不随源码授权。图标由项目作者提供，单独保留权利；详情见 [品牌资产说明](BRAND_ASSETS.md)。如果你发布衍生版本，请更换名称和图标，并保留适用的源码许可证及署名。
