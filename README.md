# 背景

当代的互联网的服务，通常都是用复杂的、大规模分布式集群来实现的。互联网应用构建在不同的软件模块集上，这些软件模块，有可能是由不同的团队开发、可能使用不同的编程语言来实现、有可能布在了几千台服务器，横跨多个不同的数据中心。因此，就需要一些可以帮助理解系统行为、用于分析性能问题的工具。

# 概览

* trace基于opentracing语义
* 全链路支持（gRPC/HTTP/MySQL/Redis/Memcached等）
 
## 参考文档

[opentracing](https://github.com/opentracing-contrib/opentracing-specification-zh/blob/master/specification.md)  
[dapper](https://bigbully.github.io/Dapper-translation/)

# 使用

本身不提供整套`trace`数据方案，但在`report.go`内声明了`repoter`接口，可以简单的集成现有开源系统，比如：`zipkin`和`jaeger`。
