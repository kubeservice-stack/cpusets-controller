# Cpusets Controller
Cpusets Controller 提供一种 Kubernetes 的设备插件，它将 CPU 内核作为可消耗设备公开给 Kubernetes 调度程序.

支持三种类型的 CPU 管理：

 - CPU独占
 - CPU共享
 - CPU默认（兼容默认core模式）
 
## cpusets controller 组件

项目包含 3 个核心组件：

1. `device plugin`: Kubernetes 标准设备插件，将 CPU 池作为可调度资源无缝集成到 Kubernetes
2. `controller`: Kubernetes 标准 Informer，确保属于不同 CPU 池的容器始终在物理上相互隔离管理和设置
3. `webhook`: 准入 Webhook，验证和变异 CPU 池特定的用户请求是否合法

* `Device Plugin` 的工作是通过现有的 `DPAPI` 将cpu分配形象作为可消耗资源通告给 Kubelet。插件分配的 CPU 作为包含物理核心 ID 列表的环境变量传递给容器。默认情况下，应用程序可以根据给定的 CPU 列表设置其进程 CPU 亲和力，或者可以将其留给标准的 Linux Completely Fair Scheduler。对于应用程序未实现设置其进程的 CPU 亲和力的功能的边缘情况，CPU 池提供了代表应用程序设置它的机制。通过将应用程序进程信息配置到其 Pod 规范的注释字段来启用此选择加入功能。

* 准入控制器 webhook，以根据启动二进制文件（安装、环境变量等）的需要来改变 Pod 的规范。

* Controller 子组件通过 Linux cpusets 实现容器的完全物理分离。通过 Informer 不断地监视 Kubernetes 的 Pod API，并在创建 Pod 或更改其状态（例如重新启动等）时触发。 shared 在共享的情况下，或者默认情况下容器没有明确要求任何池化资源。Controller 然后将计算出的集合提供给容器的 cgroupfs 文件系统 (cpuset.cpus) 的相关参数。
