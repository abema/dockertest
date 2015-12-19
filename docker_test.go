package dockertest

import (
	"fmt"
	"log"
	"testing"
)

func TestMongoDBContainer(t *testing.T) {
	fmt.Println("setup mongo container")
	con, ip, port, err := SetupMongoContainer()
	if err != nil {
		t.Fatal(err)
	}
	defer con.KillRemove()
	log.Printf("mongodb://%s:%d/abema", ip, port)
}

func TestRedisConatiner(t *testing.T) {
	fmt.Println("setup redis container")
	con, ip, port, err := SetupRedisContainer()
	if err != nil {
		panic(err)
	}
	defer con.KillRemove()
	log.Printf("%s:%d", ip, port)
}

func TestNatsContainer(t *testing.T) {
	fmt.Println("setup nats container")
	con, ip, port, err := SetupNatsContainer()
	if err != nil {
		panic(err)
	}
	defer con.KillRemove()
	log.Printf("%s:%d", ip, port)
}
