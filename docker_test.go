package dockertest

import (
	"log"
	"testing"
)

func TestMySQLContainer(t *testing.T) {
	con, ip, port, err := SetupMySQLContainer()
	if err != nil {
		t.Fatal(err)
	}
	defer con.KillRemove()
	log.Printf("tcp://%s:%d", ip, port)
}

func TestMongoDBContainer(t *testing.T) {
	con, ip, port, err := SetupMongoContainer()
	if err != nil {
		t.Fatal(err)
	}
	defer con.KillRemove()
	log.Printf("mongodb://%s:%d", ip, port)
}

func TestRedisConatiner(t *testing.T) {
	con, ip, port, err := SetupRedisContainer()
	if err != nil {
		t.Fatal(err)
	}
	defer con.KillRemove()
	log.Printf("%s:%d", ip, port)
}

func TestNatsContainer(t *testing.T) {
	con, ip, port, err := SetupNatsContainer()
	if err != nil {
		t.Fatal(err)
	}
	defer con.KillRemove()
	log.Printf("%s:%d", ip, port)
}

func TestFluentdContainer(t *testing.T) {
	con, ip, port, err := SetupFluentdContainer()
	if err != nil {
		t.Fatal(err)
	}
	defer con.KillRemove()
	log.Printf("%s:%d", ip, port)
}

func TestContainerWithArgs(t *testing.T) {
	con, ip, port, err := SetupContainer("nats", 4333, "-p", "4333")
	if err != nil {
		t.Fatal(err)
	}
	defer con.KillRemove()
	log.Printf("%s:%d", ip, port)
}
