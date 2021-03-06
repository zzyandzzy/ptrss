package main

import (
	"fmt"
	"testing"
)

func TestInstance(t *testing.T) {
	Instance()
}

func TestInit(t *testing.T) {
	Init()
}

func TestExistTable(t *testing.T) {
	fmt.Println(ExistTable("t_rss"))
}

func TestInsertClient(t *testing.T) {
	fmt.Println(InsertClient("tr", "http://192.168.1.110:9091", "", ""))
}

func TestQueryClient(t *testing.T) {
	fmt.Printf("%#v\n", QueryClient("tr"))
}

func TestQueryRSS(t *testing.T) {
	for _, rss := range QueryRSS("tr") {
		fmt.Printf("%#v\n", rss)
	}
}

func TestExistData(t *testing.T) {
	fmt.Println(ExistData("1reste4"))
}
