# Connla Windows 桌面版

这是一个独立窗口的本地知识库。完整解压发布包后，双击 `Connla.exe` 即可打开；无需先安装 Go、Node.js、Docker 或启动浏览器。首次使用按窗口提示创建管理员账号。

桌面窗口内部使用 Microsoft Edge WebView2 渲染现有界面；程序本身仍运行一个仅监听 `127.0.0.1:8081` 的本地服务。若电脑缺少 WebView2 Runtime，请按错误提示从 [微软官网](https://developer.microsoft.com/microsoft-edge/webview2/) 安装后重试。

个人数据继续保存在 `%LOCALAPPDATA%\PersonalKnowledgeBase\data`，更新或移动 `Connla.exe` 不会迁移或清空该目录。关闭 Connla 窗口会结束它启动的本地服务；如果提示端口 `8081` 已被占用，请先自行关闭旧版 Connla，本程序不会替你结束其他进程。

请保存好管理员密码，并定期按 [备份与恢复说明](https://github.com/Geronimo0830/Connla/blob/main/docs/BACKUP_RESTORE.md) 备份到另一块磁盘。首次公开版没有自动更新、安装向导或内置备份按钮；在升级前，请先关闭程序并完成备份。不要把数据目录、备份或私人文档上传到公开仓库。
