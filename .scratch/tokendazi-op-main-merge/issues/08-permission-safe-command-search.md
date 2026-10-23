# 08: 命令搜索严格复用侧边栏权限

**Spec:** `../../2026-10-23-tokendazi-op-main-merge-spec.md`


**Type:** task
**What to build:** 让命令搜索与当前侧边栏视图共享同一套角色、用户配置和嵌套工作区解析，因此不会显示用户无权访问或主动隐藏的页面，也不会出现重复菜单实例。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] Search Provider 只提供 context 与快捷键状态，不在 Provider 内重复渲染命令菜单。
- [x] 命令菜单复用统一 sidebar view，而不是维护另一套权限/配置过滤逻辑。
- [x] 根导航和嵌套工作区导航均按当前路径、角色和 sidebar modules 返回一致结果。
- [x] 命令项 key 对同标题或同 URL 的边界稳定，不产生 React key 冲突。
- [x] 访客、普通用户、管理员、隐藏模块和嵌套工作区的搜索可见性测试通过。
- [x] 如上游统一 sidebar view 与二开工作区导航规则冲突，停止并标记 `needs-info`，不得通过放宽搜索权限解决。

## Answer

- Command Menu 直接复用 `useSidebarView`，根导航、嵌套工作区、角色与 sidebar 配置采用同一过滤结果；Search Provider 仅保留 context/快捷键职责。
- 命令 key 使用标题与 URL 组合；访客、用户、管理员及隐藏模块搜索测试通过。
