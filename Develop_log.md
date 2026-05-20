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

# ingestion 流程优化

现在的 ingestion 流程

上传文档 → 解析成纯文本 → 切成 chunk → 每个 chunk 算 embedding → 存 Milvus

问题：切出来的 chunk 就是原始文本的一段。比如一段讲"退款流程"的文字，chunk 里只有那段话本身——没有标题、没有摘要、没有"这段在讲什么"的元信息。

检索时全靠这段原文的向量去匹配。如果用户问法和原文用词差很多，就召回不到。

优化思路：

将文件解析为纯文本后，做一次 LLM 加工，抽取全文关键词，生成摘要，推断元数据

切成 chunk 后，对每个 chunk 做 LLM 加工，生成该 chunk 的摘要，这段能回答什么问题，相关的关键词是什么

# llm 判断 chunk 属于哪个 intent，而不是依靠人工对一整个文档进行分类

目前的设计是在人工上传文档时，要求用户选择该文档属于哪个 intent（也就是存在哪个 partition）。如果用户上传的文档内容比较复杂， 可能同时涉及多个 intent，但用户只能选择一个，导致所有 chunk 都被放在同一个 partition 里。

导致不属于该 partition 的 chunk 被放到该 partition 里，造成检索时的噪声；同时也导致真正属于该 partition 的 chunk 没有被正确分类，造成检索时的漏召回。

优化思路：在 ingestion 时，先将文档切成 chunk，然后对每个 chunk 做 LLM 分类，判断它属于哪个 intent。这样同一文档的不同 chunk 可以被分到不同的 partition 里，更符合实际的知识结构。

# retrieval 优化

之前 retrieval 的设计是对于子问题的intent，如果 channel 允许，并且 intent 的置信度高于某个阈值，就直接在对应的 partition 里做检索；如果没有任何一个 intent 的置信度高于阈值，就在全局做检索。

问题：如果用户在上传文件的时候，没有对这个文件进行正确的分类，或者根本就没有分类，那么这个文件会进入到默认的 partition 里。在进行检索的时候，仅仅会在匹配的 partition 里进行检索，可能这个 partition 是空的，真正的相关内容被留在了默认 partition 里，导致检索不到。

优化思路：把不同优先级的 channel 进行 group，相同优先级的 channel 处于同一个 tier，然后在检索的时候，先按照优先级顺序依次在不同的 tier 里进行检索；在两个 tier 中，会记录上一次检索的结果，如果为空，则触发兜底。如果不为空，则判断是否需要继续在下个 tier 检索。
