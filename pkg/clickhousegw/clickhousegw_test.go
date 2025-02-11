package clickhousegw

import (
	"database/sql"
	"fmt"
	"testing"
	"time"
)

func TestClickHouseGateway_getCreateTableSchemaDDL(t *testing.T) {
	zookeeperPathPrefix := time.Now().Unix()
	type fields struct {
		cfg *ClickhouseConfig
		db  *sql.DB
	}
	type args struct {
		isBaseTable         bool
		zookeeperPathPrefix int64
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   string
	}{
		{
			name: "Test getCreateTableSchemaDDL for simple MergeTree",
			fields: fields{
				cfg: &ClickhouseConfig{
					Database: "test",
					Sharded:  false,
				},
			},
			args: args{
				isBaseTable:         true,
				zookeeperPathPrefix: zookeeperPathPrefix,
			},
			want: `
		CREATE TABLE IF NOT EXISTS flows (
			agent           IPv6,
			int_in          String,
			int_out         String,
			src_ip_addr     IPv6,
			dst_ip_addr     IPv6,
			src_ip_pfx_addr IPv6,
			src_ip_pfx_len  UInt8,
			dst_ip_pfx_addr IPv6,
			dst_ip_pfx_len  UInt8,
			nexthop         IPv6,
			next_asn        UInt32,
			src_asn         UInt32,
			dst_asn         UInt32,
			ip_protocol     UInt8,
			src_port        UInt16,
			dst_port        UInt16,
			timestamp       DateTime,
			size            UInt64,
			packets         UInt64,
			samplerate      UInt64
		) ENGINE = MergeTree()
		PARTITION BY toStartOfTenMinutes(timestamp)
		ORDER BY (timestamp)
		TTL timestamp + INTERVAL 14 DAY
		SETTINGS index_granularity = 8192
	`,
		},
		{
			name: "Test getCreateTableSchemaDDL for sharded base table with engine ReplicatedMergeTree",
			fields: fields{
				cfg: &ClickhouseConfig{
					Database: "test",
					Cluster:  "test_cluster",
					Sharded:  true,
				},
			},
			args: args{
				isBaseTable:         true,
				zookeeperPathPrefix: zookeeperPathPrefix,
			},
			want: fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS ON CLUSTER test_cluster _test.flows_base (
			agent           IPv6,
			int_in          String,
			int_out         String,
			src_ip_addr     IPv6,
			dst_ip_addr     IPv6,
			src_ip_pfx_addr IPv6,
			src_ip_pfx_len  UInt8,
			dst_ip_pfx_addr IPv6,
			dst_ip_pfx_len  UInt8,
			nexthop         IPv6,
			next_asn        UInt32,
			src_asn         UInt32,
			dst_asn         UInt32,
			ip_protocol     UInt8,
			src_port        UInt16,
			dst_port        UInt16,
			timestamp       DateTime,
			size            UInt64,
			packets         UInt64,
			samplerate      UInt64
		) ENGINE = ReplicatedMergeTree('/clickhouse/tables/{shard}/test/flows_%d', '{replica}')
		PARTITION BY toStartOfTenMinutes(timestamp)
		ORDER BY (timestamp)
		TTL timestamp + INTERVAL 14 DAY
		SETTINGS index_granularity = 8192
	`, zookeeperPathPrefix),
		},
		{
			name: "Test getCreateTableSchemaDDL for Distributed Table",
			fields: fields{
				cfg: &ClickhouseConfig{
					Database: "test",
					Sharded:  true,
					Cluster:  "test_cluster",
				},
			},
			args: args{
				isBaseTable:         false,
				zookeeperPathPrefix: zookeeperPathPrefix,
			},
			want: `
		CREATE TABLE IF NOT EXISTS ON CLUSTER test_cluster flows (
			agent           IPv6,
			int_in          String,
			int_out         String,
			src_ip_addr     IPv6,
			dst_ip_addr     IPv6,
			src_ip_pfx_addr IPv6,
			src_ip_pfx_len  UInt8,
			dst_ip_pfx_addr IPv6,
			dst_ip_pfx_len  UInt8,
			nexthop         IPv6,
			next_asn        UInt32,
			src_asn         UInt32,
			dst_asn         UInt32,
			ip_protocol     UInt8,
			src_port        UInt16,
			dst_port        UInt16,
			timestamp       DateTime,
			size            UInt64,
			packets         UInt64,
			samplerate      UInt64
		) ENGINE = Distributed(test_cluster, _test, flows_base, rand())
		PARTITION BY toStartOfTenMinutes(timestamp)
		ORDER BY (timestamp)
		
		SETTINGS index_granularity = 8192
	`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ClickHouseGateway{
				cfg: tt.fields.cfg,
				db:  tt.fields.db,
			}
			if got := c.getCreateTableSchemaDDL(tt.args.isBaseTable, zookeeperPathPrefix); got != tt.want {
				t.Errorf("getCreateTableSchemaDDL() = %v, want %v", got, tt.want)
			}
		})
	}
}
