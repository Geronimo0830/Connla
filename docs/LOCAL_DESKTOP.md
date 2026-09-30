# Connla Windows 桌面版

这是一个独立窗口的本地知识库。完整解压发布包后，双击 `Connla.exe` 即可打开；无需先安装 Go、Node.js、Docker 或启动浏览器。首次使用按窗口提示创建管理员账号。

桌面窗口内部使用 Microsoft Edge WebView2 渲染现有界面；程序本身仍运行一个仅监听 `127.0.0.1:8081` 的本地服务。若电脑缺少 WebView2 Runtime，请按错误提示从 [微软官网](https://developer.microsoft.com/microsoft-edge/webview2/) 安装后重试。

个人数据继续保存在 `%LOCALAPPDATA%\PersonalKnowledgeBase\data`，更新或移动 `Connla.exe` 不会迁移或清空该目录。关闭 Connla 窗口会结束它启动的本地服务；如果提示端口 `8081` 已被占用，请先自行关闭旧版 Connla，本程序不会替你结束其他进程。

管理员登录后，可以在「设置 → 备份与恢复」中完成整库备份与恢复：

- 「创建备份」：选择保存位置，程序短暂停止本地服务，保存并校验数据库和本地原始文件，然后重新启动。建议把生成的备份文件夹复制到另一块磁盘。
- 「选择备份」：选择含 `manifest.json` 的备份文件夹。确认后，程序会先为当前数据创建经过校验的安全备份，再校验并恢复所选备份。原数据和安全备份会留在 `%LOCALAPPDATA%\PersonalKnowledgeBase\backups`，不会被直接删除；完成后界面会重新加载，可能需要重新登录。

仅桌面版的管理员能看到这个设置入口。从源码在浏览器里运行时，仍可使用[命令行备份与恢复说明](https://github.com/Geronimo0830/Connla/blob/main/docs/BACKUP_RESTORE.md)。请保存好管理员密码；不要把数据目录、备份或私人文档上传到公开仓库。当前没有自动更新或安装向导。
