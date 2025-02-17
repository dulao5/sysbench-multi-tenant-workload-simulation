package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// TableInfo holds the metadata of a table, including name and the value range of column 'k'.
type TableInfo struct {
	Name string
	MinK int
	MaxK int
}

type SysbenchRow struct {
	ID  int    `gorm:"primaryKey;autoIncrement"`
	K   int    `gorm:"index:k_1"`
	C   string `gorm:"size:120"`
	Pad string `gorm:"size:60"`
}

func main() {
	// Parse command-line flags
	var (
		// Number of databases (default: 10): test0001 ~ test0010
		dbNum = flag.Int("db-num", 10, "Number of databases (default: 10)")

		// Number of rows per big table (default: 10000)
		rowsPerBigTable = flag.Int("rows-per-big-table", 10000, "Rows per big table (default: 10000)")
		// Number of big tables (default: 67)
		bigTableNum = flag.Int("big-table-num", 67, "Number of big tables (default: 67)")

		// Number of rows per small table (default: 900)
		rowsPerSmallTable = flag.Int("rows-per-small-table", 900, "Rows per small table (default: 900)")
		// Number of small tables (default: 334)
		smallTableNum = flag.Int("small-table-num", 334, "Number of small tables (default: 334)")

		// Total rows of each small partition table (default: 334800)
		// e.g. 372 partitions * 900 rows each = 334800
		rowsPerSmallPartitionTable = flag.Int("rows-pre-small-partition-tables", 334800, "Rows per small partition table in total (default: 334800)")
		// Number of small partition tables (default: 3)
		smallPartitionTableNum = flag.Int("small-partition-table-num", 3, "Number of small partition tables (default: 3)")

		// Number of threads per DB (default: 17)
		threadsPerDB = flag.Int("threads-pre-db", 17, "Threads (long connections) per DB (default: 17)")

		// Sleep duration in milliseconds after each query (default: 359)
		sleepAfterQueryMs = flag.Int("sleep-after-query-ms", 359, "Sleep duration in ms after each query (default: 359)")

		// DSN prefix, e.g. root:@tcp(127.0.0.1:4000)/
		// The actual dbName will be appended when opening a specific DB.
		dsn = flag.String("dsn", "root:@tcp(127.0.0.1:4000)/", "Data Source Name prefix for MySQL/TiDB")

		// testing time seconds (default: 600 seconds)
		testingTimeSeconds = flag.Int("testing-time-seconds", 600, "testing time seconds (default: 600 seconds)")
	)
	flag.Parse()

	var exitTime = time.Now().Add(time.Second * time.Duration(*testingTimeSeconds))

	// Prepare table information (big tables, small tables, small partition tables).
	tables := prepareTables(*bigTableNum, *rowsPerBigTable,
		*smallTableNum, *rowsPerSmallTable,
		*smallPartitionTableNum, *rowsPerSmallPartitionTable)

	log.Printf("[INFO] Starting workload with %d DB(s), each DB has %d threads ...\n", *dbNum, *threadsPerDB)

	var wg sync.WaitGroup

	// For each database, create a separate *sql.DB instance and launch goroutines.
	for dbIndex := 1; dbIndex <= *dbNum; dbIndex++ {
		dbName := fmt.Sprintf("test%04d", dbIndex) // e.g. test0001, test0002, etc.
		dbDSN := *dsn + dbName

		// Open a database handle.
		// Note: By default, sql.DB is a connection pool manager.
		//       We'll get a dedicated *sql.Conn from it in each goroutine.
		dbConn, err := sql.Open("mysql", dbDSN)
		if err != nil {
			log.Fatalf("[ERROR] Failed to open DB %s: %v", dbName, err)
		}

		// Optional: Set connection pool parameters if needed.
		// Example: Use the same number for max open/idle as threadsPerDB,
		//          so that each thread can hold one dedicated connection.
		// dbConn.SetMaxOpenConns(*threadsPerDB)
		// dbConn.SetMaxIdleConns(*threadsPerDB)

		// Ping test to ensure the DB is reachable.
		if err := dbConn.Ping(); err != nil {
			log.Fatalf("[ERROR] Failed to ping DB %s: %v", dbName, err)
		}
		log.Printf("[INFO] DB %s connected", dbName)

		// Launch 'threadsPerDB' goroutines (long connections).
		for i := 0; i < *threadsPerDB; i++ {
			wg.Add(1)
			time.Sleep(50 * time.Millisecond)
			go func(conn *sql.DB, dbName string) {
				defer wg.Done()
				runWorker(conn, dbName, tables, *sleepAfterQueryMs, exitTime)
			}(dbConn, dbName)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for all goroutines to finish (though in this case they run indefinitely).
	wg.Wait()
	log.Printf("[INFO] Stop workload with %d DB(s) x %d threads\n", *dbNum, *threadsPerDB)
}

// prepareTables creates the TableInfo list based on the given parameters.
func prepareTables(bigTableNum, rowsPerBigTable int,
	smallTableNum, rowsPerSmallTable int,
	smallPartitionTableNum, rowsPerSmallPartitionTable int) []TableInfo {

	tables := make([]TableInfo, 0, bigTableNum+smallTableNum+smallPartitionTableNum)

	// 1. Big tables: sbtest001 ~ sbtest067
	for i := 1; i <= bigTableNum; i++ {
		// tableName := fmt.Sprintf("sbtest%03d", i)
		tableName := fmt.Sprintf("sbtest%d", i)
		tables = append(tables, TableInfo{
			Name: tableName,
			MinK: 1,
			MaxK: rowsPerBigTable,
		})
	}

	// 2. Small tables: sbtest068 ~ sbtest(067 + 334) = sbtest401
	startSmallTableIndex := bigTableNum + 1
	endSmallTableIndex := bigTableNum + smallTableNum
	for i := startSmallTableIndex; i <= endSmallTableIndex; i++ {
		//tableName := fmt.Sprintf("sbtest%03d", i)
		tableName := fmt.Sprintf("sbtest%d", i)
		tables = append(tables, TableInfo{
			Name: tableName,
			MinK: 1,
			MaxK: rowsPerSmallTable,
		})
	}

	// 3. Small partition tables: sbtest402 ~ sbtest404
	startSmallPartitionIndex := endSmallTableIndex + 1
	endSmallPartitionIndex := endSmallTableIndex + smallPartitionTableNum
	for i := startSmallPartitionIndex; i <= endSmallPartitionIndex; i++ {
		// tableName := fmt.Sprintf("sbtest%03d", i)
		tableName := fmt.Sprintf("sbtest%d", i)
		tables = append(tables, TableInfo{
			Name: tableName,
			MinK: 1,
			MaxK: rowsPerSmallPartitionTable,
		})
	}
	return tables
}

func makeActiveConn(db *sql.DB, dbName string, ctx context.Context) (*sql.Conn, error) {
	log.Printf("[INFO] get conn for DB %s", dbName)
	conn, err := db.Conn(ctx)
	if err != nil {
		log.Printf("[ERROR] Failed to get conn for DB %s: %v", dbName, err)
		return nil, err
	}
	err = conn.PingContext(ctx)
	if err != nil {
		log.Printf("[ERROR] Failed to ping conn for DB %s: %v", dbName, err)
		return nil, err
	}
	return conn, nil
}

func retryMakeActiveConn(db *sql.DB, dbName string, ctx context.Context) (*sql.Conn, error) {
	for {
		conn, err := makeActiveConn(db, dbName, ctx)
		if err != nil {
			log.Printf("[WARNING] retry conn for DB %s: %v", dbName, err)
			time.Sleep(50 * time.Millisecond)
		} else {
			return conn, nil
		}
	}
}

// runWorker gets one sql.Conn from the pool and continuously performs queries on that single connection.
func runWorker(dbConn *sql.DB, dbName string, tables []TableInfo, sleepMs int, exitTime time.Time) {
	// Get a dedicated connection from the pool.
	ctx := context.Background()
	conn, err := retryMakeActiveConn(dbConn, dbName, ctx)
	if err != nil {
		log.Printf("[ERROR] Failed to get conn for DB %s: %v", dbName, err)
		return
	}
	defer conn.Close()

	// do a join select sql
	_ = doJoinSelectRawDB(conn, ctx, 900)

	// Infinite loop to continuously send queries.
	for {
		// Randomly pick a table
		tableInfo := tables[rand.Intn(len(tables))]

		// Generate a random 'k' value within [MinK, MaxK]
		kVal := rand.Intn(tableInfo.MaxK-tableInfo.MinK+1) + tableInfo.MinK

		// Build the query: SELECT c FROM sbtestXYZ WHERE k=? LIMIT 1
		query := fmt.Sprintf("SELECT c FROM %s WHERE k=? LIMIT 1", tableInfo.Name)

		// Measure query time
		start := time.Now()

		if start.After(exitTime) {
			break
		}

		// Use QueryRowContext on the single *sql.Conn
		row := conn.QueryRowContext(ctx, query, kVal)

		var cVal string
		err := row.Scan(&cVal)
		duration := time.Since(start)

		// If there's an error and it's not a "no rows" case, log it.
		if err != nil && err != sql.ErrNoRows {
			log.Printf("[ERROR] DB=%s table=%s k=%d query failed: %v", dbName, tableInfo.Name, kVal, err)
			conn, _ = retryMakeActiveConn(dbConn, dbName, ctx)
		} else {
			// Optionally log or collect the duration metrics here.
			// log.Printf("[INFO] DB=%s table=%s k=%d took=%v", dbName, tableInfo.Name, kVal, duration)
			_ = duration
		}

		// Sleep to control QPS
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)
	}
}

func doJoinSelectRawDB(conn *sql.Conn, ctx context.Context, maxId uint64) error {
	// do Join select query
	// table : sysbench.sbtest1
	// id: 1~maxID
	// limit 100
	// left join : sbtest1 , sbtest2 , sbtest3 , sbtest4 on `id` colunm (as same value)
	randID := uint64(rand.Int63n(int64(maxId)-100)) + 1
	var result SysbenchRow

	/*
	       rows, err := conn.QueryContext(
	           ctx,
	           `select (sbtest1.id + sbtest5.id + sbtest6.id + sbtest7.id + sbtest8.id) as id, sbtest2.k as k, sbtest3.c as c, sbtest4.pad as pad
	   from sbtest1
	   LEFT JOIN sbtest2 ON sbtest1.id = sbtest2.id
	   LEFT JOIN sbtest3 ON sbtest1.id = sbtest3.id
	   LEFT JOIN sbtest4 ON sbtest1.id = sbtest4.id
	   LEFT JOIN sbtest5 ON sbtest1.id = sbtest5.id
	   LEFT JOIN sbtest6 ON sbtest1.id = sbtest6.id
	   LEFT JOIN sbtest7 ON sbtest1.id = sbtest7.id
	   LEFT JOIN sbtest8 ON sbtest1.id = sbtest8.id
	   Where sbtest1.id >= ?
	   limit 300`,
	           randID,
	       )
	*/
	rows, err := conn.QueryContext(
		ctx,
		`select (sbtest1.id) as id, sbtest2.k as k, sbtest3.c as c, sbtest4.pad as pad
from sbtest1
LEFT JOIN sbtest2 ON sbtest1.id = sbtest2.id
LEFT JOIN sbtest3 ON sbtest1.id = sbtest3.id
LEFT JOIN sbtest4 ON sbtest1.id = sbtest4.id
Where sbtest1.id >= ?
limit 100`,
		randID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	// result
	for rows.Next() {
		if err := rows.Scan(&result.ID, &result.K, &result.C, &result.Pad); err != nil {
			return err
		}
	}
	return nil
}
