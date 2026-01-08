package initial

import (
	"context"
	"fmt"
	"strings"

	"OmniLink/internal/config"
	"OmniLink/pkg/zlog"

	mclient "github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

var MilvusClient mclient.Client

func init() {
	conf := config.GetConfig()
	addr := strings.TrimSpace(conf.MilvusConfig.Address)
	if addr == "" {
		return
	}

	ctx := context.Background()
	cli, err := newMilvusClientAndEnsureSchema(ctx, conf)
	if err != nil {
		zlog.Fatal(fmt.Sprintf("milvus init failed: %v", err))
		return
	}
	MilvusClient = cli
}

func newMilvusClientAndEnsureSchema(ctx context.Context, conf *config.Config) (mclient.Client, error) {
	addr := strings.TrimSpace(conf.MilvusConfig.Address)
	dbName := strings.TrimSpace(conf.MilvusConfig.DBName)
	collection := strings.TrimSpace(conf.MilvusConfig.CollectionName)

	if dbName == "" {
		dbName = "omnilink"
	}
	if collection == "" {
		collection = "ai_kb_vectors"
	}

	dim := conf.MilvusConfig.VectorDim
	if dim <= 0 {
		dim = 768
	}

	defaultCli, err := mclient.NewClient(ctx, mclient.Config{
		Address:  addr,
		Username: strings.TrimSpace(conf.MilvusConfig.Username),
		Password: strings.TrimSpace(conf.MilvusConfig.Password),
		DBName:   "default",
	})
	if err != nil {
		return nil, err
	}

	dbs, err := defaultCli.ListDatabases(ctx)
	if err != nil {
		_ = defaultCli.Close()
		return nil, err
	}
	exists := false
	for _, db := range dbs {
		if db.Name == dbName {
			exists = true
			break
		}
	}
	if !exists {
		if err := defaultCli.CreateDatabase(ctx, dbName); err != nil {
			_ = defaultCli.Close()
			return nil, err
		}
	}

	cli, err := mclient.NewClient(ctx, mclient.Config{
		Address:  addr,
		Username: strings.TrimSpace(conf.MilvusConfig.Username),
		Password: strings.TrimSpace(conf.MilvusConfig.Password),
		DBName:   dbName,
	})
	if err != nil {
		_ = defaultCli.Close()
		return nil, err
	}

	cols, err := cli.ListCollections(ctx)
	if err != nil {
		_ = defaultCli.Close()
		_ = cli.Close()
		return nil, err
	}
	collExists := false
	for _, c := range cols {
		if c.Name == collection {
			collExists = true
			break
		}
	}

	if !collExists {
		schema := &entity.Schema{
			CollectionName: collection,
			Description:    "OmniLink AI knowledge base vectors",
			Fields: []*entity.Field{
				{
					Name:       "id",
					DataType:   entity.FieldTypeVarChar,
					PrimaryKey: true,
					TypeParams: map[string]string{"max_length": "128"},
				},
				{
					Name:       "vector",
					DataType:   entity.FieldTypeFloatVector,
					TypeParams: map[string]string{entity.TypeParamDim: fmt.Sprintf("%d", dim)},
				},
				{
					Name:     "tenant_user_id",
					DataType: entity.FieldTypeVarChar,
					TypeParams: map[string]string{
						"max_length": "20",
					},
				},
				{
					Name:     "kb_id",
					DataType: entity.FieldTypeInt64,
				},
				{
					Name:     "source_type",
					DataType: entity.FieldTypeVarChar,
					TypeParams: map[string]string{
						"max_length": "30",
					},
				},
				{
					Name:     "source_key",
					DataType: entity.FieldTypeVarChar,
					TypeParams: map[string]string{
						"max_length": "128",
					},
				},
				{
					Name:     "chunk_id",
					DataType: entity.FieldTypeInt64,
				},
				{
					Name:     "content",
					DataType: entity.FieldTypeVarChar,
					TypeParams: map[string]string{
						"max_length": "4096",
					},
				},
				{
					Name:     "metadata",
					DataType: entity.FieldTypeJSON,
				},
			},
		}

		if err := cli.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
			_ = defaultCli.Close()
			_ = cli.Close()
			return nil, err
		}

		idx, err := entity.NewIndexAUTOINDEX(entity.COSINE)
		if err != nil {
			_ = defaultCli.Close()
			_ = cli.Close()
			return nil, err
		}
		if err := cli.CreateIndex(ctx, collection, "vector", idx, false); err != nil {
			_ = defaultCli.Close()
			_ = cli.Close()
			return nil, err
		}
	}

	_ = defaultCli.Close()

	_ = cli.LoadCollection(ctx, collection, false)

	return cli, nil
}
