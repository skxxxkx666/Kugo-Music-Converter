//go:build !windows

package main

// 非 Windows 平台不提供原生集成（单实例、任务栏进度、Toast 通知）。
// 这些方法与 app_windows_integration.go 中的实现一一对应，保证跨平台编译。

func enforceSingleInstance(_ string) (release func(), ok bool) {
	return func() {}, true
}

func (a *App) initNativeIntegration() {}

func (a *App) nativeTaskStart() {}

func (a *App) nativeTaskProgress(_ int) {}

func (a *App) nativeTaskComplete(_ bool, _ int, _ int) {}

func (a *App) nativeTaskFailed() {}
