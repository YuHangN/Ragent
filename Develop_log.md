# 核心设计

## Intent-Aware Retrieval Routing

设计并实现了一套面向 Agentic RAG 的意图感知检索架构。

系统会根据用户问题动态选择检索策略。例如，当用户询问“退款流程”时，系统会通过 Intent Routing 将检索范围缩小到 FAQ Partition，仅搜索相关知识域；而当用户提出“整个系统如何保证一致性”这类跨领域或语义模糊的问题时，则自动切换为 Global Semantic Search，对整个 KB 执行全局向量检索作为兜底策略。

底层基于 Milvus Collection + Partition 实现逻辑知识隔离：
- Collection 作为 KB 的物理向量空间
- Partition 作为不同 Intent 的逻辑知识域
- Chunk 仅存储一份，避免 embedding 重复与多 collection 管理复杂度

# Retrieval Routing 核心逻辑

当用户提出一个问题后，系统首先会根据对话历史对问题进行 Query Rewrite，将上下文相关的问题重写为更完整、独立的查询。

如果重写后的问题较复杂，系统可能会进一步拆分为多个子问题。随后，每个子问题都会独立进行 Intent Resolution，判断它可能属于哪些 KB Intent。一个子问题可能命中一个或多个 Intent，每个 Intent 都会带有对应的置信度分数。

检索阶段以“子问题”为基本单位进行路由：

- 如果某个子问题存在一个或多个高置信度 KB Intent，则该子问题会进入 Intent-Directed Retrieval。
    - 系统会使用该子问题作为 query。
    - 对每一个高置信度 Intent 对应的 collection / partition 分别执行一次定向检索。
    - 低置信度 Intent 会被忽略，不会参与检索。
    - 只要该子问题至少有一个高置信度 KB Intent，就认为它已经被 Direct 覆盖，不再触发 Global 兜底检索。

- 如果某个子问题的所有 Intent 置信度都低，或者没有命中 KB Intent，则该子问题会进入 Global Retrieval。
    - 系统会使用该子问题作为 query。
    - 在用户指定的 KB 默认 collection 中执行全局语义检索，作为兜底召回。

因此，一个用户问题最终会被展开为多个检索任务。部分子问题可能走 Direct，部分子问题可能走 Global；而同一个子问题如果命中多个高置信度 Intent，也可能产生多次 Direct 检索。

整体逻辑可以概括为：

`Query Rewrite → Query Decomposition → Per-SubQuestion Intent Resolution → Direct / Global Retrieval Routing → Merge / Dedup / Rerank → LLM Answer`

这个设计避免了复杂问题只做一次全局向量检索导致的语义混杂问题，同时通过高置信度 Intent 缩小检索范围，提高精准度；对于无法明确分类的子问题，又通过 Global Retrieval 保留兜底召回能力。