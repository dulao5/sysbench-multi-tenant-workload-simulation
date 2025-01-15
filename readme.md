# TiDB Workload (MySQL Protocol) in Go

This project provides a sample workload generator for TiDB (MySQL protocol).  
It simulates random queries using the `k` index on various tables,  
with **long connections** (each thread keeps using the same `sql.Conn`).

---

## Testing Scenario

In this scenario, we have **10 databases** (`test0001 ~ test0010` by default).  
Each database contains the following tables:

1. **Big Tables**  
   - **Table Name**: `sbtest1` ~ `sbtest67`  
   - **Total**: 67 big tables  
   - **Rows per table**: 10,000  
   - **Schema (example)**:
     ```sql
     CREATE TABLE sbtest1 (
       id  INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
       k   INT NOT NULL DEFAULT 0,
       c   VARCHAR(120) NOT NULL DEFAULT '',
       pad VARCHAR(60) NOT NULL DEFAULT ''
     );
     ```
   - (Similarly for `sbtest1` ~ `sbtest67` with the same structure, each holding 10,000 rows.)

2. **Small Tables**  
   - **Table Name**: `sbtest68` ~ `sbtest401`  
   - **Total**: 334 small tables  
   - **Rows per table**: 900  
   - **Schema (example)**:
     ```sql
     CREATE TABLE sbtest68 (
       id  INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
       k   INT NOT NULL DEFAULT 0,
       c   VARCHAR(120) NOT NULL DEFAULT '',
       pad VARCHAR(60) NOT NULL DEFAULT ''
     );
     ```
   - (Likewise for `sbtest68` ~ `sbtest401`, each holding 900 rows.)

3. **Small Partition Tables**  
   - **Table Name**: `sbtest402` ~ `sbtest404`  
   - **Total**: 3 partitioned tables  
   - **Partitions**: 372 partitions per table  
   - **Rows per partition**: 900  
     - So each table has `372 * 900 = 334,800` rows in total.  
   - **Schema (example)**:
     ```sql
     CREATE TABLE sbtest402 (
       id  INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
       k   INT NOT NULL DEFAULT 0,
       c   VARCHAR(120) NOT NULL DEFAULT '',
       pad VARCHAR(60) NOT NULL DEFAULT ''
     )
     PARTITION BY HASH (k)
     PARTITIONS 372;
     ```
   - (Similarly for `sbtest403` and `sbtest404`, each has 372 partitions.)

> **Note**: The above schema is indicative.  
> You mentioned `k mod 372` for partitioning, which effectively matches `PARTITION BY HASH(k)` with 372 partitions.  
> Data preparation is assumed to have been done beforehand.

---

## How It Works

1. For **each database** (`test0001 ~ test0010` by default), the program opens a `sql.DB` handle (connection pool).  
2. **`-threads-pre-db`** determines how many goroutines (threads) will be launched for each DB.  
3. Each goroutine obtains **a dedicated `sql.Conn`** from the pool (`dbConn.Conn(...)`) and then runs in a loop:  
   - Randomly pick one table (from big, small, or small partition tables).  
   - Randomly generate a `k` within the valid range for that table.  
   - Execute `SELECT c FROM sbtestXYZ WHERE k=? LIMIT 1` using `QueryRowContext`.  
   - Sleep for a specified duration (default 359ms) to control the QPS.  
4. Because each thread keeps the same `sql.Conn`, connections are not reclaimed by the pool until the goroutine finishes.

---

## Usage

### 1. Initialize

```bash
go mod init tidb-workload
go get github.com/go-sql-driver/mysql
```

### 2. Build
```
go build -o workload .
```

### 3. Run
```
./workload \
  -dsn="root:@tcp(127.0.0.1:4000)/" \
  -db-num=10 \
  -rows-per-big-table=10000 \
  -big-table-num=67 \
  -rows-per-small-table=900 \
  -small-table-num=334 \
  -rows-pre-small-partition-tables=334800 \
  -small-partition-table-num=3 \
  -threads-pre-db=17 \
  -sleep-after-query-ms=359
```

### Command Line Flags

*	-dsn
The DSN prefix for MySQL/TiDB.
Must end with /, because the code will append the database name (e.g. test0001).
*	-db-num
Number of databases to simulate (test0001, test0002, …, test0010).
*	-rows-per-big-table / -big-table-num
Control how many rows in each “big table” and how many such tables.
*	-rows-per-small-table / -small-table-num
Control rows in each “small table” and how many such tables.
*	-rows-pre-small-partition-tables / -small-partition-table-num
Control total rows for each “small partition table” and how many such tables exist.
*	-threads-pre-db
Number of goroutines (long connections) per database.
*	-sleep-after-query-ms
Sleep time in milliseconds after each query (to control QPS).


### Notes > Data Preparation:
You should have already created databases test0001 ~ test0010,
and created and loaded data into each table according to the specs described above.
This tool does not create the tables or load data; it only runs queries.

