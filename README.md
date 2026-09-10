# 隅读

Windows 下的 TXT 悬浮阅读器。拖动边缘改大小，用方向键翻页，按老板键隐藏窗口。适用于 Windows 10 / 11，64 位。

## 下载和使用

从 [Releases](https://github.com/Sphenomenon/float-reader/releases/latest) 下载 Windows 压缩包，解压后双击 `FloatReader.exe`。不用安装。包里附了一篇示例小说，也可以把自己的 TXT 拖进窗口。

![阅读窗口](docs/screenshots/阅读窗口.png)

| 操作 | 默认快捷键 |
| --- | --- |
| 下一页 | → 或 ↓ |
| 上一页 | ← 或 ↑ |
| 隐藏 / 显示 | Ctrl + Alt + 空格 |
| 导入 TXT | Ctrl + O |
| 打开设置 | Ctrl + , |

右上角的“设置”可以改快捷键、每页字数、窗口尺寸和外观。阅读位置会自动保存，下次打开接着读。

TXT 按块读取，翻页时只保留当前页需要的文字。背景不透明度和文字不透明度分开设置：背景调到 1% 或 0%，正文仍按文字不透明度显示。背景为 0% 时，窗口的标题、按钮和底栏会完全隐去，只留下正文；右键正文可以移动窗口或重新打开设置。

老板键在其他软件里也能用。翻页键默认只在阅读窗口里生效，想在别的软件里翻页，需要开启全局翻页。隐藏后，翻页键会释放给其他软件。

找不到窗口时，点右下角托盘图标。退出程序也在托盘的右键菜单里。更多用法见[使用说明](docs/使用说明.md)。

## 编译

需要 Go 1.23 或更新版本，没有第三方 Go 依赖。

Linux / macOS：

```sh
bash scripts/build.sh
```

这个脚本还需要 Python 3，用来生成 Windows 资源和压缩包。输出在 `dist/`。

Windows PowerShell：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/build.ps1
```

PowerShell 脚本生成 `dist/FloatReader.exe`。需要压缩包时，再运行 `python scripts/package.py`。

仓库带有编译好的 `.syso` 资源。修改图标、版本或 manifest 后，运行 `python3 scripts/resources.py` 重新生成。

## 测试

目前在 Linux + Wine 11.0 下测试，Windows 机器上还没测。[测试记录](docs/测试记录.md)里有运行命令、报告和截图。

置顶不能覆盖 UAC 安全提示、锁屏或独占全屏；其他置顶窗口也可能暂时盖住阅读器。发布的 EXE 未做代码签名，下载页附有 SHA-256 校验文件。

程序不联网。设置和阅读位置保存在 `%APPDATA%\FloatReader\settings.json`，原 TXT 文件不会被修改。
