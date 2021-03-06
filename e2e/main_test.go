package main_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

var pool *dockertest.Pool
var resources []*dockertest.Resource

func TestMain(m *testing.M) {
	code := 0

	// defer teardown
	defer teardownMain(&code)

	// setup
	setupMain()

	// run
	log.Println("running tests...")
	code = m.Run()
}

func setupMain() {
	log.Println("setting up tests...")

	var err error
	pool, err = dockertest.NewPool("")
	if err != nil {
		log.Panicf("could not connect to docker: %s", err)
	}

	// start mongo container
	resource, err := pool.Run("mongo", "latest", []string{
		"MONGO_INITDB_ROOT_USERNAME=mongo",
		"MONGO_INITDB_ROOT_PASSWORD=mongo",
	})
	if err != nil {
		log.Panicf("could not start mongo container: %s", err)
	}

	// add resource to resources
	resources = append(resources, resource)

	// format mongo connection uri
	uri := fmt.Sprintf(
		"mongodb://mongo:mongo@%s/mongo?authSource=admin",
		resource.GetHostPort("27017/tcp"),
	)

	// log.Printf("uri: %s\n", uri)

	// set mongo connection uri
	os.Setenv("MONGO_CONNECTION_URI", uri)

	// exponential backoff-retry until container ready to accept connections
	if err := pool.Retry(func() error {
		ctx := context.Background()
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
		if err != nil {
			return err
		}
		return client.Ping(ctx, readpref.Primary())
	}); err != nil {
		log.Panicf("could not connect to mongo container: %s", err)
	}
}

func teardownMain(code *int) {
	log.Println("tearing down tests...")

	if r := recover(); r != nil {
		log.Printf("recovered from panic: %s\n", r)
		debug.PrintStack()
	}

	if pool != nil {
		for _, resource := range resources {
			if err := pool.Purge(resource); err != nil {
				log.Panicf("could not purge resource: %s", err)
			}
		}
	}

	os.Exit(*code)
}

// helpers

func panicGuard(t *testing.T) {
	if r := recover(); r != nil {
		t.Errorf("recovered from panic: %s\n", r)
		debug.PrintStack()
	}
}

func newTestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
