# 隅读 · Float Reader

一个用于 Windows 10 / 11 x64 的无边框悬浮 TXT 小说阅读器。原生 Win32 + Go，单个 EXE，无安装步骤，不需要管理员权限，不依赖 .NET、WebView2 或 Python。应用本身不联网。

## 下载与使用

从 [GitHub Releases](https://github.com/Sphenomenon/float-reader/releases/latest) 下载 Windows 使用包，解压后双击 `FloatReader.exe`。压缩包附带中文使用说明和示例小说。

默认使用方向键翻页，**Ctrl + Alt + 空格**隐藏 / 显示；点击右上角“设置”修改快捷键、字数、尺寸和外观。

![阅读窗口](docs/screenshots/阅读窗口.png)

## 本地构建产物

仓库保存源码、资源、文档和构建脚本；Windows EXE 与 ZIP 发布在 Releases。运行构建脚本后生成以下本地文件：

- `dist/隅读-1.0.0-Windows-x64.zip`：客户使用包，解压后运行 `FloatReader.exe`。
- `dist/FloatReader.exe`：可直接复制使用的独立程序。
- `dist/隅读-1.0.0-源码.zip`：完整源码与构建脚本。
- [中文使用说明](docs/使用说明.md)
- [测试记录](docs/测试记录.md)

## 功能

- TXT 文件选择、拖放导入、命令行文件路径导入；UTF-8、UTF-8 BOM、UTF-16 LE/BE、GB18030 / GBK 自动识别，单文件上限 32 MB。
- 无标准标题栏和关闭按钮；拖动顶部移动，拖动四边或四角调整大小。
- Windows `HWND_TOPMOST` 置顶，并定期恢复置顶顺序，保持阅读窗口可见且不抢焦点。
- 按实际字体测量分页，自动换行；拉伸窗口后重新分页并保留阅读位置。
- 设置目标字数时自动匹配窗口尺寸，并设置每页字数上限；手动拉伸后恢复按区域自适应。段落换行会占用行高，实际字数可能低于目标，文字始终以能完整显示为准。
- 自定义上一页、下一页、老板键、导入、打开设置五个快捷键；检测重复和系统占用。
- 老板键全局生效，同时隐藏阅读窗口、设置与文件选择窗口；相同按键恢复。隐藏时释放全局翻页键。
- 可选全局翻页、三种配色、字号、行距、不透明度、数字宽高设置。
- 托盘恢复、最近阅读、阅读位置与设置自动保存、单实例运行。

## 默认快捷键

| 操作 | 按键 |
| --- | --- |
| 下一页 | →；默认配置下 ↓ 也可用 |
| 上一页 | ←；默认配置下 ↑ 也可用 |
| 隐藏 / 显示 | Ctrl + Alt + Space，始终全局生效 |
| 导入 TXT | Ctrl + O |
| 设置 | Ctrl + , |

默认翻页快捷键只在阅读器获得焦点时使用。设置中开启“在其他软件中也能用翻页快捷键”后，可在其他软件获得焦点时翻页。全局模式使用设置中录入的两个翻页键；上下方向键仅为默认局部快捷键的便捷别名。

## 构建

源码只使用 Go 标准库。需要 Go 1.23 或更高版本。已经附带可直接用于构建的 Windows `.syso` 资源；修改图标、版本或 manifest 时，用 Python 3 运行 `python3 scripts/resources.py` 重新生成。

Linux / macOS：

```sh
bash scripts/build.sh
```

Windows PowerShell：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build.ps1
```

单独编译：

```sh
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags "-H windowsgui -s -w" -o dist/FloatReader.exe ./cmd/floatreader
```

## 验证

```sh
go test ./...
GOOS=windows GOARCH=amd64 go vet -unsafeptr=false ./...
wine dist/FloatReader.exe --smoke-test
```

Windows 下直接运行 `FloatReader.exe --smoke-test` 也可执行原生窗口检查。测试会模拟键盘操作，使用独立配置，并在 EXE 同目录的 `qa/` 中生成报告和截图；任一检查失败时进程返回非零。测试期间应让程序完成，不要手动操作测试窗口。

`go vet` 关闭 `unsafeptr` 是因为 Win32 回调必须将系统提供的 `LPARAM` 转回结构体指针，其余检查正常启用。向系统调用传递指针时使用带 `uintptrescapes` 的封装，保持 Go 内存生命周期正确。

## 实现与范围

- `internal/reader`：平台无关的 Unicode 文本规范化、分页和阅读锚点逻辑。
- `internal/win32`：Win32 API、结构体与窗口绘制封装。
- `cmd/floatreader`：原生阅读窗口、设置、托盘、编码识别、热键和实际窗口测试。
- `scripts/resources.py`：无外部依赖的 COFF 资源生成器。

Windows 置顶不覆盖 UAC 安全桌面、锁屏或独占全屏应用，这是系统权限边界；其他置顶窗口也可能临时位于前面。目标为 Windows 10 / 11 x64，不提供 Windows 7、32 位系统或原生 ARM64 构建。

配置保存在 `%APPDATA%\FloatReader\settings.json`。只保存设置、文件路径与阅读位置，不复制或修改原始小说。删除程序即可卸载；需要清除设置时，退出程序后删除该配置目录。

交付二进制尚未进行 Authenticode 商业代码签名；企业客户如要求签名分发，可使用其代码签名证书签署成品。
