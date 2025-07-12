# xxl-job-execute-go 重制版

声明：本案思路，实现的基本逻辑，设计等，还是来源于 `xxl-job-execute-go`，由于老项目年久失修，加之本人也联系不上原作者，故此重制。

## 主要改动点

1. 修复了原库中Task被其他routing修改的bug
2. 适配xxl-job 3.1.1，考虑维护性删除对xxl-job 2 版本的支持
3. 调整TaskFunc为·func(context.Context, *Task) error·
4. 统一调整返回值和java版本一致（便于核对，维护）
5. 移除原库Log的实现，替换成标注库中的slog
6. go.mod 升级到1.23
7. 移除ipv4的依赖，现在需要明确设置执行器的URL
8. 移除Executor接口，直接返回 *Executor
9. 提供Executor.Handle()方法，返回一个http.Handle方便与其他web框架集成

## 后续计划和调整

- [ ] 输出日志还是延续原来做法，后续会被调整
- [ ] 提供一个默认的基于slog输出日志文件的logHandle 便于从xxl-job管理端查看日志
- [ ] 对老代码风格统一调整

## 一些需要注意且很重要的问题

1. 本案移除了ExecutorIp和ExecutorPort 两个参数，取而代之的是提供了ExecuorURL用来设置本地执行器URL，这个URl会上报个xxl-job-admin，这个参数必须设置，没有默认值（请注意，如果URL中含有path，也请将Handle注册到对应的path上，本库没有提供任何辅助方法，需要你自行处理）

2. KillParam 中只有JobID，没有其他参数，这导致第一个任务同时执行时候，取消会发生问题，***这不是bug***，我和xxl-job的作者沟通过此问题希望加入LogID，但xxl-job原作者表示设计如此
