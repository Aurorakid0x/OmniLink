Retriever - Milvus v2 (推荐)
向量数据库 Milvus 版介绍

向量检索服务 Milvus 版为基于开源 Milvus 构建的全托管数据库服务，提供高效的非结构化数据检索能力，适用于多样化 AI 场景，客户无需再关心底层硬件资源，降低使用成本，提高整体效率。

鉴于公司内场的 Milvus 服务采用标准 SDK，因此适用 EINO-ext 社区版本。

本包为 EINO 框架提供 Milvus 2.x (V2 SDK) 检索器实现，支持多种搜索模式的向量相似度搜索。

注意: 本包需要 Milvus 2.5+ 以支持服务器端函数（如 BM25），基础功能兼容低版本。

功能特性
Milvus V2 SDK: 使用最新的 milvus-io/milvus/client/v2 SDK
多种搜索模式: 支持近似搜索、范围搜索、混合搜索、迭代器搜索和标量搜索
稠密 + 稀疏混合搜索: 结合稠密向量和稀疏向量，使用 RRF 重排序
自定义结果转换: 可配置的结果到文档转换
安装
go get github.com/cloudwego/eino-ext/components/retriever/milvus2
快速开始
package main

import (
        "context"
        "fmt"
        "log"
        "os"

        "github.com/cloudwego/eino-ext/components/embedding/ark"
        "github.com/milvus-io/milvus/client/v2/milvusclient"

        milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
        "github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
)

func main() {
        // 获取环境变量
        addr := os.Getenv("MILVUS_ADDR")
        username := os.Getenv("MILVUS_USERNAME")
        password := os.Getenv("MILVUS_PASSWORD")
        arkApiKey := os.Getenv("ARK_API_KEY")
        arkModel := os.Getenv("ARK_MODEL")

        ctx := context.Background()

        // 创建 embedding 模型
        emb, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
                APIKey: arkApiKey,
                Model:  arkModel,
        })
        if err != nil {
                log.Fatalf("Failed to create embedding: %v", err)
                return
        }

        // 创建 retriever
        retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
                ClientConfig: &milvusclient.ClientConfig{
                        Address:  addr,
                        Username: username,
                        Password: password,
                },
                Collection: "my_collection",
                TopK:       10,
                SearchMode: search_mode.NewApproximate(milvus2.COSINE),
                Embedding:  emb,
        })
        if err != nil {
                log.Fatalf("Failed to create retriever: %v", err)
                return
        }
        log.Printf("Retriever created successfully")

        // 检索文档
        documents, err := retriever.Retrieve(ctx, "search query")
        if err != nil {
                log.Fatalf("Failed to retrieve: %v", err)
                return
        }

        // 打印文档
        for i, doc := range documents {
                fmt.Printf("Document %d:\n", i)
                fmt.Printf("  ID: %s\n", doc.ID)
                fmt.Printf("  Content: %s\n", doc.Content)
                fmt.Printf("  Score: %v\n", doc.Score())
        }
}
配置选项
字段	类型	默认值	描述
Client
*milvusclient.Client
-	预配置的 Milvus 客户端（可选）
ClientConfig
*milvusclient.ClientConfig
-	客户端配置（Client 为空时必需）
Collection
string
"eino_collection"
集合名称
TopK
int
5
返回结果数量
VectorField
string
"vector"
稠密向量字段名
SparseVectorField
string
"sparse_vector"
稀疏向量字段名
OutputFields
[]string
所有字段	结果中返回的字段
SearchMode
SearchMode
-	搜索策略（必需）
Embedding
embedding.Embedder
-	用于查询向量化的 Embedder（必需）
DocumentConverter
func
默认转换器	自定义结果到文档转换
ConsistencyLevel
ConsistencyLevel
ConsistencyLevelDefault
一致性级别 (
ConsistencyLevelDefault
使用 collection 的级别；不应用按请求覆盖)
Partitions
[]string
-	要搜索的分区
搜索模式
从 github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode 导入搜索模式。

近似搜索 (Approximate)
标准的近似最近邻 (ANN) 搜索。

mode := search_mode.NewApproximate(milvus2.COSINE)
范围搜索 (Range)
在指定距离范围内搜索 (向量在 Radius 内)。

// L2: 距离 <= Radius
// IP/Cosine: 分数 >= Radius
mode := search_mode.NewRange(milvus2.L2, 0.5).
    WithRangeFilter(0.1) // 可选: 环形搜索的内边界
稀疏搜索 (BM25)
使用 BM25 进行纯稀疏向量搜索。需要 Milvus 2.5+ 支持稀疏向量字段并启用 Functions。

// 纯稀疏搜索 (BM25) 需要指定 OutputFields 以获取内容
// MetricType: BM25 (默认) 或 IP
mode := search_mode.NewSparse(milvus2.BM25)

// 在配置中，使用 "*" 或特定字段以确保返回内容:
// OutputFields: []string{"*"}
混合搜索 (Hybrid - 稠密 + 稀疏)
结合稠密向量和稀疏向量的多向量搜索，支持结果重排序。需要一个同时包含稠密和稀疏向量字段的集合（参见 indexer sparse 示例）。

import (
    "github.com/milvus-io/milvus/client/v2/milvusclient"
    milvus2 "github.com/cloudwego/eino-ext/components/retriever/milvus2"
    "github.com/cloudwego/eino-ext/components/retriever/milvus2/search_mode"
)

// 定义稠密 + 稀疏子请求的混合搜索
hybridMode := search_mode.NewHybrid(
    milvusclient.NewRRFReranker().WithK(60), // RRF 重排序器
    &search_mode.SubRequest{
        VectorField: "vector",             // 稠密向量字段
        VectorType:  milvus2.DenseVector,  // 默认值，可省略
        TopK:        10,
        MetricType:  milvus2.L2,
    },
    // 稀疏子请求 (Sparse SubRequest)
    &search_mode.SubRequest{
        VectorField: "sparse_vector",       // 稀疏向量字段
        VectorType:  milvus2.SparseVector,  // 指定稀疏类型
        TopK:        10,
        MetricType:  milvus2.BM25,          // 使用 BM25 或 IP
    },
)

// 创建 retriever (稀疏向量生成由 Milvus Function 服务器端处理)
retriever, err := milvus2.NewRetriever(ctx, &milvus2.RetrieverConfig{
    ClientConfig:      &milvusclient.ClientConfig{Address: "localhost:19530"},
    Collection:        "hybrid_collection",
    VectorField:       "vector",             // 默认稠密字段
    SparseVectorField: "sparse_vector",      // 默认稀疏字段
    TopK:              5,
    SearchMode:        hybridMode,
    Embedding:         denseEmbedder,        // 稠密向量的标准 Embedder
})
迭代器搜索 (Iterator)
基于批次的遍历，适用于大结果集。

[!WARNING]

Iterator 模式的 Retrieve 方法会获取 所有 结果，直到达到总限制 (TopK) 或集合末尾。对于极大数据集，这可能会消耗大量内存。

// 100 是批次大小 (每次网络调用的条目数)
mode := search_mode.NewIterator(milvus2.COSINE, 100).
    WithSearchParams(map[string]string{"nprobe": "10"})

// 使用 RetrieverConfig.TopK 设置总限制 (IteratorLimit)。
标量搜索 (Scalar)
仅基于元数据过滤，不使用向量相似度（将过滤表达式作为查询）。

mode := search_mode.NewScalar()

// 使用过滤表达式查询
docs, err := retriever.Retrieve(ctx, `category == "electronics" AND year >= 2023`)
稠密向量度量 (Dense)
度量类型	描述
L2
欧几里得距离
IP
内积
COSINE
余弦相似度
稀疏向量度量 (Sparse)
度量类型	描述
BM25
Okapi BM25 (BM25 搜索必需)
IP
内积 (适用于预计算的稀疏向量)
二进制向量度量 (Binary)
度量类型	描述
HAMMING
汉明距离
JACCARD
杰卡德距离
TANIMOTO
Tanimoto 距离
SUBSTRUCTURE
子结构搜索
SUPERSTRUCTURE
超结构搜索
重要提示: SearchMode 中的度量类型必须与创建集合时使用的索引度量类型一致。

示例
查看 https://github.com/cloudwego/eino-ext/tree/main/components/retriever/milvus2/examples 录获取完整的示例代码：

approximate - 基础 ANN 搜索
range - 范围搜索示例
hybrid - 混合多向量搜索 (稠密 + BM25)
hybrid_chinese - 中文混合搜索示例
iterator - 批次迭代器搜索
scalar - 标量/元数据过滤
grouping - 分组搜索结果
filtered - 带过滤的向量搜索
sparse - 纯稀疏搜索示例 (BM25)
获取帮助
[集团内部版] Milvus 快速入门
如果有任何问题 或者任何功能建议，欢迎进这个群 oncall。

外部参考
Milvus 文档
Milvus 索引类型
Milvus 度量类型
Milvus Go SDK 参考
相关文档
Eino: Indexer 使用说明
Eino: Retriever 使用说明


Indexer - Milvus v2 (推荐)
向量数据库 Milvus 版介绍

向量检索服务 Milvus 版为基于开源 Milvus 构建的全托管数据库服务，提供高效的非结构化数据检索能力，适用于多样化 AI 场景，客户无需再关心底层硬件资源，降低使用成本，提高整体效率。

鉴于公司内场的 Milvus 服务采用标准 SDK，因此适用 EINO-ext 社区版本。

本包为 EINO 框架提供 Milvus 2.x (V2 SDK) 索引器实现，支持文档存储和向量索引。

注意: 本包需要 Milvus 2.5+ 以支持服务器端函数（如 BM25），基础功能兼容低版本。

功能特性
Milvus V2 SDK: 使用最新的 milvus-io/milvus/client/v2 SDK
灵活的索引类型: 支持多种索引构建器，包括 Auto, HNSW, IVF 系列, SCANN, DiskANN, GPU 索引以及 RaBitQ (Milvus 2.6+)
混合搜索就绪: 原生支持稀疏向量 (BM25/SPLADE) 与稠密向量的混合存储
服务端向量生成: 使用 Milvus Functions (BM25) 自动生成稀疏向量
自动化管理: 自动处理集合 Schema 创建、索引构建和加载
字段分析: 可配置的文本分析器（支持中文 Jieba、英文、Standard 等）
自定义文档转换: Eino 文档到 Milvus 列的灵活映射
安装
go get github.com/cloudwego/eino-ext/components/indexer/milvus2
快速开始
package main

import (
        "context"
        "log"
        "os"

        "github.com/cloudwego/eino-ext/components/embedding/ark"
        "github.com/cloudwego/eino/schema"
        "github.com/milvus-io/milvus/client/v2/milvusclient"

        milvus2 "github.com/cloudwego/eino-ext/components/indexer/milvus2"
)

func main() {
        // 获取环境变量
        addr := os.Getenv("MILVUS_ADDR")
        username := os.Getenv("MILVUS_USERNAME")
        password := os.Getenv("MILVUS_PASSWORD")
        arkApiKey := os.Getenv("ARK_API_KEY")
        arkModel := os.Getenv("ARK_MODEL")

        ctx := context.Background()

        // 创建 embedding 模型
        emb, err := ark.NewEmbedder(ctx, &ark.EmbeddingConfig{
                APIKey: arkApiKey,
                Model:  arkModel,
        })
        if err != nil {
                log.Fatalf("Failed to create embedding: %v", err)
                return
        }

        // 创建索引器
        indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
                ClientConfig: &milvusclient.ClientConfig{
                        Address:  addr,
                        Username: username,
                        Password: password,
                },
                Collection:   "my_collection",

                Vector: &milvus2.VectorConfig{
                        Dimension:  1024, // 与 embedding 模型维度匹配
                        MetricType: milvus2.COSINE,
                        IndexBuilder: milvus2.NewHNSWIndexBuilder().WithM(16).WithEfConstruction(200),
                },
                Embedding:    emb,
        })
        if err != nil {
                log.Fatalf("Failed to create indexer: %v", err)
                return
        }
        log.Printf("Indexer created successfully")

        // 存储文档
        docs := []*schema.Document{
                {
                        ID:      "doc1",
                        Content: "Milvus is an open-source vector database",
                        MetaData: map[string]any{
                                "category": "database",
                                "year":     2021,
                        },
                },
                {
                        ID:      "doc2",
                        Content: "EINO is a framework for building AI applications",
                },
        }
        ids, err := indexer.Store(ctx, docs)
        if err != nil {
                log.Fatalf("Failed to store: %v", err)
                return
        }
        log.Printf("Store success, ids: %v", ids)
}
配置选项
字段	类型	默认值	描述
Client
*milvusclient.Client
-	预配置的 Milvus 客户端（可选）
ClientConfig
*milvusclient.ClientConfig
-	客户端配置（Client 为空时必需）
Collection
string
"eino_collection"
集合名称
Vector
*VectorConfig
-	稠密向量配置 (维度, MetricType, 字段名)
Sparse
*SparseVectorConfig
-	稀疏向量配置 (MetricType, 字段名)
IndexBuilder
IndexBuilder
AutoIndexBuilder
索引类型构建器
Embedding
embedding.Embedder
-	用于向量化的 Embedder（可选）。如果为空，文档必须包含向量 (BYOV)。
ConsistencyLevel
ConsistencyLevel
ConsistencyLevelDefault
一致性级别 (
ConsistencyLevelDefault
使用 Milvus 默认: Bounded; 如果未显式设置，则保持集合级别设置)
PartitionName
string
-	插入数据的默认分区
EnableDynamicSchema
bool
false
启用动态字段支持
Functions
[]*entity.Function
-	Schema 函数定义（如 BM25），用于服务器端处理
FieldParams
map[string]map[string]string
-	字段参数配置（如 enable_analyzer）
稠密向量配置 (VectorConfig)
字段	类型	默认值	描述
Dimension
int64
-	向量维度 (必需)
MetricType
MetricType
L2
相似度度量类型 (L2, IP, COSINE 等)
VectorField
string
"vector"
稠密向量字段名
稀疏向量配置 (SparseVectorConfig)
字段	类型	默认值	描述
VectorField
string
"sparse_vector"
稀疏向量字段名
MetricType
MetricType
BM25
相似度度量类型
Method
SparseMethod
SparseMethodAuto
生成方法 (
SparseMethodAuto
或
SparseMethodPrecomputed
)
注意: 仅当 MetricType 为 BM25 时，Method 默认为 Auto。Auto 意味着使用 Milvus 服务器端函数（远程函数）。对于其他度量类型（如 IP），默认为 Precomputed。

索引构建器
稠密索引构建器 (Dense)
构建器	描述	关键参数
NewAutoIndexBuilder()
Milvus 自动选择最优索引	-
NewHNSWIndexBuilder()
基于图的高性能索引	
M
,
EfConstruction
NewIVFFlatIndexBuilder()
基于聚类的搜索	
NList
NewIVFPQIndexBuilder()
乘积量化，内存高效	
NList
,
M
,
NBits
NewIVFSQ8IndexBuilder()
标量量化	
NList
NewIVFRabitQIndexBuilder()
IVF + RaBitQ 二进制量化 (Milvus 2.6+)	
NList
NewFlatIndexBuilder()
暴力精确搜索	-
NewDiskANNIndexBuilder()
面向大数据集的磁盘索引	-
NewSCANNIndexBuilder()
高召回率的快速搜索	
NList
,
WithRawDataEnabled
NewBinFlatIndexBuilder()
二进制向量的暴力搜索	-
NewBinIVFFlatIndexBuilder()
二进制向量的聚类搜索	
NList
NewGPUBruteForceIndexBuilder()
GPU 加速暴力搜索	-
NewGPUIVFFlatIndexBuilder()
GPU 加速 IVF_FLAT	-
NewGPUIVFPQIndexBuilder()
GPU 加速 IVF_PQ	-
NewGPUCagraIndexBuilder()
GPU 加速图索引 (CAGRA)	
IntermediateGraphDegree
,
GraphDegree
稀疏索引构建器 (Sparse)
构建器	描述	关键参数
NewSparseInvertedIndexBuilder()
稀疏向量倒排索引	
DropRatioBuild
NewSparseWANDIndexBuilder()
稀疏向量 WAND 算法	
DropRatioBuild
示例：HNSW 索引
indexBuilder := milvus2.NewHNSWIndexBuilder().
        WithM(16).              // 每个节点的最大连接数 (4-64)
        WithEfConstruction(200) // 索引构建时的搜索宽度 (8-512)
示例：IVF_FLAT 索引
indexBuilder := milvus2.NewIVFFlatIndexBuilder().
        WithNList(256) // 聚类单元数量 (1-65536)
示例：IVF_PQ 索引（内存高效）
indexBuilder := milvus2.NewIVFPQIndexBuilder().
        WithNList(256). // 聚类单元数量
        WithM(16).      // 子量化器数量
        WithNBits(8)    // 每个子量化器的位数 (1-16)
示例：SCANN 索引（高召回率快速搜索）
indexBuilder := milvus2.NewSCANNIndexBuilder().
        WithNList(256).           // 聚类单元数量
        WithRawDataEnabled(true)  // 启用原始数据进行重排序
示例：DiskANN 索引（大数据集）
indexBuilder := milvus2.NewDiskANNIndexBuilder() // 基于磁盘，无额外参数
示例：Sparse Inverted Index (稀疏倒排索引)
indexBuilder := milvus2.NewSparseInvertedIndexBuilder().
        WithDropRatioBuild(0.2) // 构建时忽略小值的比例 (0.0-1.0)
稠密向量度量 (Dense)
度量类型	描述
L2
欧几里得距离
IP
内积
COSINE
余弦相似度
稀疏向量度量 (Sparse)
度量类型	描述
BM25
Okapi BM25 (
SparseMethodAuto
必需)
IP
内积 (适用于预计算的稀疏向量)
二进制向量度量 (Binary)
度量类型	描述
HAMMING
汉明距离
JACCARD
杰卡德距离
TANIMOTO
Tanimoto 距离
SUBSTRUCTURE
子结构搜索
SUPERSTRUCTURE
超结构搜索
稀疏向量支持
索引器支持两种稀疏向量模式：自动生成 (Auto-Generation) 和 预计算 (Precomputed)。

自动生成 (BM25)
使用 Milvus 服务器端函数从内容字段自动生成稀疏向量。

要求: Milvus 2.5+
配置: 设置 MetricType: milvus2.BM25。
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    // ... 基础配置 ...
    Collection:        "hybrid_collection",
    
    Sparse: &milvus2.SparseVectorConfig{
        VectorField: "sparse_vector",
        MetricType:  milvus2.BM25, 
        // BM25 时 Method 默认为 SparseMethodAuto
    },
    
    // BM25 的分析器配置
    FieldParams: map[string]map[string]string{
        "content": {
            "enable_analyzer": "true",
            "analyzer_params": `{"type": "standard"}`, // 中文使用 {"type": "chinese"}
        },
    },
})
预计算 (SPLADE, BGE-M3 等)
允许存储由外部模型（如 SPLADE, BGE-M3）或自定义逻辑生成的稀疏向量。

配置: 设置 MetricType（通常为 IP）和 Method: milvus2.SparseMethodPrecomputed。
用法: 通过 doc.WithSparseVector() 传入稀疏向量。
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    Collection: "sparse_collection",
    
    Sparse: &milvus2.SparseVectorConfig{
        VectorField: "sparse_vector",
        MetricType:  milvus2.IP,
        Method:      milvus2.SparseMethodPrecomputed,
    },
})

// 存储包含稀疏向量的文档
doc := &schema.Document{ID: "1", Content: "..."}
doc.WithSparseVector(map[int]float64{
    1024: 0.5,
    2048: 0.3,
})
indexer.Store(ctx, []*schema.Document{doc})
自带向量 (Bring Your Own Vectors)
如果您的文档已经包含向量，可以不配置 Embedder 使用 Indexer。

// 创建不带 embedding 的 indexer
indexer, err := milvus2.NewIndexer(ctx, &milvus2.IndexerConfig{
    ClientConfig: &milvusclient.ClientConfig{
        Address: "localhost:19530",
    },
    Collection:   "my_collection",
    Vector: &milvus2.VectorConfig{
        Dimension:  128,
        MetricType: milvus2.L2,
    },
    // Embedding: nil, // 留空
})

// 存储带有预计算向量的文档
docs := []*schema.Document{
    {
        ID:      "doc1",
        Content: "Document with existing vector",
    },
}

// 附加稠密向量到文档
// 向量维度必须与集合维度匹配
vector := []float64{0.1, 0.2, ...} 
docs[0].WithDenseVector(vector)

// 附加稀疏向量（可选，如果配置了 Sparse）
// 稀疏向量是 index -> weight 的映射
sparseVector := map[int]float64{
    10: 0.5,
    25: 0.8,
}
docs[0].WithSparseVector(sparseVector)

ids, err := indexer.Store(ctx, docs)
对于 BYOV 模式下的稀疏向量，请参考上文 预计算 (Precomputed) 部分进行配置。

示例
查看 https://github.com/cloudwego/eino-ext/tree/main/components/indexer/milvus2/examples 目录获取完整的示例代码：

demo - 使用 HNSW 索引的基础集合设置
hnsw - HNSW 索引示例
ivf_flat - IVF_FLAT 索引示例
rabitq - IVF_RABITQ 索引示例 (Milvus 2.6+)
auto - AutoIndex 示例
diskann - DISKANN 索引示例
hybrid - 混合搜索设置 (稠密 + BM25 稀疏) (Milvus 2.5+)
hybrid_chinese - 中文混合搜索示例 (Milvus 2.5+)
sparse - 纯稀疏索引示例 (BM25)
byov - 自带向量示例
获取帮助
[集团内部版] Milvus 快速入门
如果有任何问题 或者任何功能建议，欢迎进这个群 oncall。

外部参考
Milvus 文档
Milvus 索引类型
Milvus 度量类型
Milvus Go SDK 参考
相关文档
Eino: Indexer 使用说明
Eino: Retriever 使用说明