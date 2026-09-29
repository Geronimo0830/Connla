# Connla Windows 本地版使用说明

这是单机使用的知识库。运行时不需要 Docker、Go 或 Node.js；它只监听 `127.0.0.1:8081`，不会主动开放给局域网。

## 首次启动

1. 将整个 `connla-windows` 文件夹放在你希望保存程序的位置。不要只移动 `Connla.exe`。
2. 双击 `start.cmd`。浏览器会打开 `http://127.0.0.1:8081/`。
3. 首次进入时按页面提示建立管理员账号。请保存好密码。

程序数据固定放在 `%LOCALAPPDATA%\PersonalKnowledgeBase\data`，不在程序文件夹中。你移动或更新程序文件夹时，原始文件和数据库不会随之移动或被清空。日志在同一数据目录的 `server.out.log` 和 `server.err.log`。

## 日常操作

- 启动：双击 `start.cmd`。如果已经运行，只会重新打开浏览器。
- 停止：双击 `stop.cmd`。关闭浏览器不会停止程序。
- 备份：先运行 `stop.cmd`，再运行 `backup.cmd`。备份位于 `%LOCALAPPDATA%\PersonalKnowledgeBase\backups` 中的新时间戳目录；不会覆盖旧备份。备份后可再次运行 `start.cmd`。

备份包含 SQLite 数据库、数据库引用的本地原始文件和配置。请把重要备份额外复制到另一块磁盘，并加密保存。若使用 S3 或把附件设置到数据目录之外，内置本地备份会拒绝，必须单独备份外部文件。不要把数据目录或备份目录上传到公开仓库。

## 恢复与升级

恢复前先停止程序。参考程序文件夹中的 `BACKUP_RESTORE.md`，使用 `Connla.exe backup restore --input <备份目录> --target <全新目录>`。恢复命令不会覆盖当前数据；先在独立目录检查原始文件、文档和搜索结果，再决定是否切换数据。不要把备份直接解压到正在使用的数据目录。

升级程序时先停止，做一次备份，再用新版 `connla-windows` 程序文件夹替换旧程序文件夹。数据目录保持不动。首次升级后的数据库迁移可能无法用旧程序读取，所以保留升级前的备份。

如果启动失败，先检查 `server.err.log`；如果提示端口 `8081` 已占用，请先确认是否有另一份知识库或其他程序正在监听该端口，不要结束不认识的进程。

## 从源码重新打包

在项目根目录运行 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts\package-local-windows.ps1`。脚本先构建网页，再构建 Windows 程序，输出到被 Git 忽略的 `build\connla-windows`。本机需要 Node.js 24、pnpm，以及 Go 1.27 或已启动的 Docker Desktop。重新打包不会读取或删除个人数据目录。
