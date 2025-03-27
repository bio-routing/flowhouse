package clickhousegw

import (
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/bio-routing/flowhouse/pkg/models/flow"
	"github.com/pkg/errors"

	"github.com/ClickHouse/clickhouse-go"

	bnet "github.com/bio-routing/bio-rd/net"
)

const tableName = "flows"

// ClickHouseGateway is a wrapper for Clickhouse
type ClickHouseGateway struct {
	cfg *ClickhouseConfig
	db  *sql.DB
}

// ClickhouseConfig represents a clickhouse client config
type ClickhouseConfig struct {
	Host     string `yaml:"host"`
	Address  string `yaml:"address"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	Sharded  bool   `yaml:"sharded"`
	Cluster  string `yaml:"cluster"`
	Secure   bool   `yaml:"secure"`
}

// New instantiates a new ClickHouseGateway
func New(cfg *ClickhouseConfig) (*ClickHouseGateway, error) {
	dsn := fmt.Sprintf("tcp://%s?username=%s&password=%s&database=%s&read_timeout=10&write_timeout=20&secure=%t",
		cfg.Address, cfg.User, cfg.Password, cfg.Database, cfg.Secure)
	c, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, errors.Wrap(err, "sql.Open failed")
	}

	err = c.Ping()
	if err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			return nil, errors.Wrapf(err, "[%d] %s \n%s", exception.Code, exception.Message, exception.StackTrace)
		}

		return nil, errors.Wrap(err, "c.Ping failed")
	}

	chgw := &ClickHouseGateway{
		cfg: cfg,
		db:  c,
	}

	err = chgw.createFlowsSchemaIfNotExists()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create flows schema")
	}

	return chgw, nil
}

func (c *ClickHouseGateway) createFlowsSchemaIfNotExists() error {
	zookeeperPathTimestamp := time.Now().Unix()
	_, err := c.db.Exec(c.getCreateTableSchemaDDL(true, zookeeperPathTimestamp))

	if err != nil {
		return errors.Wrap(err, "Query failed")
	}

	if c.cfg.Sharded {
		_, err = c.db.Exec(c.getCreateTableSchemaDDL(false, zookeeperPathTimestamp))
	}
	if err != nil {
		return errors.Wrap(err, "Query failed")
	}

	return nil
}

func (c *ClickHouseGateway) getCreateTableSchemaDDL(isBaseTable bool, zookeeperPathPrefix int64) string {
	tableDDl := `
		CREATE TABLE IF NOT EXISTS%s %s (
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
		) ENGINE = %s
		PARTITION BY toStartOfTenMinutes(timestamp)
		ORDER BY (timestamp)
		%s
		SETTINGS index_granularity = 8192
	`
	ttl := "TTL timestamp + INTERVAL 14 DAY"
	onClusterStatement := ""
	if c.cfg.Sharded {
		onClusterStatement = " ON CLUSTER " + c.cfg.Cluster
	}

	if isBaseTable {
		return fmt.Sprintf(tableDDl, onClusterStatement, c.getBaseTableName(), c.getBaseTableEngineDDL(zookeeperPathPrefix), ttl)
	} else {
		return fmt.Sprintf(tableDDl, onClusterStatement, tableName, c.getDistributedTableDDl(), "")
	}
}

func (c *ClickHouseGateway) getBaseTableName() string {
	if c.cfg.Sharded {
		return "_" + c.cfg.Database + "." + tableName + "_base"
	}

	return tableName
}

func (c *ClickHouseGateway) getBaseTableEngineDDL(zookeeperPathPrefix int64) string {
	if c.cfg.Sharded {
		// TODO: make zookeeper path configurable
		return fmt.Sprintf(
			"ReplicatedMergeTree('/clickhouse/tables/{shard}/%s/%s_%d', '{replica}')",
			c.cfg.Database,
			tableName,
			zookeeperPathPrefix)
	}

	return "MergeTree()"
}
func (c *ClickHouseGateway) getDistributedTableDDl() string {
	return fmt.Sprintf(
		"Distributed(%s, %s, %s, %s)",
		c.cfg.Cluster,
		"_"+c.cfg.Database,
		tableName+"_base",
		"rand()")
}

// InsertFlows inserts flows into clickhouse
func (c *ClickHouseGateway) InsertFlows(flows []*flow.Flow) error {
	tx, err := c.db.Begin()
	if err != nil {
		return errors.Wrap(err, "Begin failed")
	}

	stmt, err := tx.Prepare(`INSERT INTO flows (
		agent, 
		int_in, 
		int_out, 
		src_ip_addr, 
		dst_ip_addr, 
		src_ip_pfx_addr, 
		src_ip_pfx_len, 
		dst_ip_pfx_addr, 
		dst_ip_pfx_len, 
		nexthop, 
		next_asn, 
		src_asn, 
		dst_asn, 
		ip_protocol, 
		src_port, 
		dst_port, 
		timestamp, 
		size, 
		packets, 
		samplerate
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ? , ?, ?, ?)`)
	if err != nil {
		return errors.Wrap(err, "Prepare failed")
	}

	defer stmt.Close()

	for _, fl := range flows {
		_, err := stmt.Exec(
			fl.Agent.ToNetIP(),
			fl.IntIn,
			fl.IntOut,
			fl.SrcAddr.ToNetIP(),
			fl.DstAddr.ToNetIP(),
			addrToNetIP(fl.SrcPfx.Addr()),
			fl.SrcPfx.Pfxlen(),
			addrToNetIP(fl.DstPfx.Addr()),
			fl.DstPfx.Pfxlen(),
			fl.NextHop.ToNetIP(),
			fl.NextAs,
			fl.SrcAs,
			fl.DstAs,
			fl.Protocol,
			fl.SrcPort,
			fl.DstPort,
			fl.Timestamp,
			fl.Size,
			fl.Packets,
			fl.Samplerate,
		)
		if err != nil {
			return errors.Wrap(err, "Exec failed")
		}
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "Commit failed")
	}

	return nil
}

func addrToNetIP(addr *bnet.IP) net.IP {
	if addr == nil {
		return net.IP([]byte{0, 0, 0, 0})
	}

	return addr.ToNetIP()
}

// Close closes the database handler
func (c *ClickHouseGateway) Close() {
	c.db.Close()
}

// GetColumnValues gets all unique values of a column
func (c *ClickHouseGateway) GetColumnValues(columnName string) ([]string, error) {
	columnName = strings.Replace(columnName, " ", "", -1)

	query := fmt.Sprintf("SELECT %s FROM flows GROUP BY %s", columnName, columnName)
	res, err := c.db.Query(query)

	if err != nil {
		return nil, errors.Wrap(err, "Exec failed")
	}

	result := make([]string, 0)

	for {
		v := ""
		res.Scan(&v)

		result = append(result, v)
		if !res.Next() {
			break
		}
	}

	return result, nil
}

// GetDictValues gets all values of a certain dicts attribute
func (c *ClickHouseGateway) GetDictValues(dictName string, attr string) ([]string, error) {
	dictName = strings.Replace(dictName, " ", "", -1)
	attr = strings.Replace(attr, " ", "", -1)

	query := fmt.Sprintf("SELECT %s FROM dictionary(%s) GROUP BY %s", attr, dictName, attr)
	res, err := c.db.Query(query)

	if err != nil {
		return nil, errors.Wrap(err, "Exec failed")
	}

	result := make([]string, 0)

	for {
		v := ""
		res.Scan(&v)

		result = append(result, v)
		if !res.Next() {
			break
		}
	}

	return result, nil
}

// GetDictFields gets the names of all fields in a dictionary
func (c *ClickHouseGateway) GetDictFields(dictName string) ([]string, error) {
	dictName = strings.Replace(dictName, " ", "", -1)

	query := fmt.Sprintf("SELECT attribute.names FROM system.dictionaries WHERE name = '%s';", dictName)
	res, err := c.db.Query(query)
	if err != nil {
		return nil, errors.Wrap(err, "Exec failed")
	}

	result := make([]string, 0)
	res.Next()
	err = res.Scan(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// DescribeTable gets the names of all fields of a table
func (c *ClickHouseGateway) DescribeTable(tableName string) ([]string, error) {
	tableName = strings.Replace(tableName, " ", "", -1)

	query := fmt.Sprintf("DESCRIBE %s", tableName)
	res, err := c.db.Query(query)

	if err != nil {
		return nil, errors.Wrap(err, "Exec failed")
	}

	result := make([]string, 0)

	for {
		name := ""
		trash := ""
		res.Scan(&name, &trash, &trash, &trash, &trash, &trash, &trash)

		result = append(result, name)
		if !res.Next() {
			break
		}
	}

	return result, nil
}

// GetDatabaseName gets the databases name
func (c *ClickHouseGateway) GetDatabaseName() string {
	return c.cfg.Database
}

// Query executs an SQL query
func (c *ClickHouseGateway) Query(q string) (*sql.Rows, error) {
	return c.db.Query(q)
}
