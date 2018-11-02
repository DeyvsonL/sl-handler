package database

import (
	"database/sql"
	"fmt"

	//_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	connection *sql.DB
}

type Function struct {
	id     int
	name   string
	cpus   int
	memory int
	code   string
	pack   string
}

const (
	//sqliteVersion = "mysql"
	//pathDataBase  = "root:password@/app_development"
	sqliteVersion = "sqlite3"
	pathDataBase  = "./database.db"
)

func (d *Database) Connect() {
	var connection, err = sql.Open(sqliteVersion, pathDataBase)
	checkErr(err)
	d.connection = connection
	d.createSchema()
}

func (d *Database) Close() {
	d.connection.Close()
}

func (d *Database) createSchema() {
	fmt.Println("TESTE")
	switch sqliteVersion {
	case "mysql":
		fmt.Println("MYSQL")
		var qFunctionTable = "CREATE TABLE IF NOT EXISTS function (id INT(10) NOT NULL AUTO_INCREMENT, name TEXT, cpus INTEGER, memory INTEGER, code TEXT, pack TEXT, PRIMARY KEY (`id`))"
		statement, _ := d.connection.Prepare(qFunctionTable)
		statement.Exec()

		var qMetricTable = "CREATE TABLE IF NOT EXISTS metric (id INT(10) NOT NULL AUTO_INCREMENT, function_id TEXT, duration INTEGER, status_code INTEGER, throttler, concurrentExecutions INTEGER, PRIMARY KEY (`id`))"
		d.connection.Prepare(qMetricTable)
		statement.Exec()

	case "sqlite3":
		fmt.Println("SQLITE3")
		var qFunctionTable = "CREATE TABLE IF NOT EXISTS function (id INTEGER PRIMARY KEY, name TEXT, cpus INTEGER, memory INTEGER, code TEXT, pack TEXT)"
		statement, _ := d.connection.Prepare(qFunctionTable)
		statement.Exec()

		var qMetricTable = "CREATE TABLE IF NOT EXISTS metric (id INTEGER PRIMARY KEY, function_id TEXT, duration INTEGER, status_code INTEGER, throttler, concurrentExecutions INTEGER)"
		statement, _ = d.connection.Prepare(qMetricTable)
		statement.Exec()
	}
}

func (d *Database) InsertFunction(name string, cpus, memory int, code string, pack string) {
	statement, err := d.connection.Prepare("INSERT INTO function (name, cpus, memory, code, pack) VALUES (?, ?, ?, ?, ?)")
	checkErr(err)
	_, err = statement.Exec(name, cpus, memory, code, pack)
	checkErr(err)
}

func (d *Database) DeleteFunction(id int) {
	statement, err := d.connection.Prepare("DELETE FROM function WHERE id=?")
	checkErr(err)

	_, err = statement.Exec(id)
	checkErr(err)
}

func (d *Database) SelectFunction(name string) {
	statement, err := d.connection.Prepare("SELECT FROM function WHERE name=?")
	checkErr(err)

	_, err = statement.Exec(name)
	checkErr(err)
}

func (d *Database) SelectAllFunction() []Function {
	rows, err := d.connection.Query("SELECT * FROM function")
	checkErr(err)
	var functionList = make([]Function, 0)

	for rows.Next() {
		function := Function{}
		err = rows.Scan(&function.id, &function.name, &function.cpus, &function.memory, &function.code, &function.pack)
		checkErr(err)
		functionList = append(functionList, function)
	}

	return functionList
}

func (d *Database) SelectByNameFunction(name string) []Function {
	rows, err := d.connection.Query(fmt.Sprintf("SELECT * FROM function WHERE name='%v'", name))
	checkErr(err)
	var functionList = make([]Function, 0)

	for rows.Next() {
		function := Function{}
		err = rows.Scan(&function.id, &function.name, &function.cpus, &function.memory, &function.code, &function.pack)
		checkErr(err)
		functionList = append(functionList, function)
	}

	return functionList
}

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}
