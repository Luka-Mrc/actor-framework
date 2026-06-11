// Command coordinator pokrece Coordinator (+ Aggregator/Evaluator/Logger) kao
// zaseban proces i slusa gRPC. Konfiguracija je env-driven (vidi docker-compose.yml).
package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/lukam/actor-framework/federated"
	"github.com/lukam/actor-framework/federated/data"
	"github.com/lukam/actor-framework/federated/model"
	"github.com/lukam/actor-framework/framework"
	"github.com/lukam/actor-framework/framework/remote"
)

func main() {
	listen := env("LISTEN", ":9000")
	advertised := env("ADVERTISED", "coordinator:9000")
	expected := envInt("EXPECTED_TRAINERS", 3)
	rounds := envInt("TOTAL_ROUNDS", 5)
	dataDir := env("DATA_DIR", "/data")
	seed := int64(envInt("SEED", 42))

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	trainPath := filepath.Join(dataDir, "KDDTrain+.txt")
	testPath := filepath.Join(dataDir, "KDDTest+.txt")

	train, err := data.LoadTrain(trainPath)
	if err != nil {
		logger.Error("load train", "path", trainPath, "error", err.Error())
		os.Exit(1)
	}
	test, err := data.LoadTest(testPath, train.Schema)
	if err != nil {
		logger.Error("load test", "path", testPath, "error", err.Error())
		os.Exit(1)
	}
	initial := model.NewWeights(train.InputDim(), 64, 32, data.NumClasses, seed)
	logger.Info("coordinator up",
		"listen", listen, "advertised", advertised,
		"expected_trainers", expected, "rounds", rounds,
		"input_dim", train.InputDim(), "test_size", test.Len())

	sys := framework.NewActorSystem("coordinator",
		framework.WithLogger(logger),
		framework.WithAdvertisedHost(advertised))
	rs := remote.NewRemoteSystem(sys, listen)
	rs.SetAdvertisedAddress(advertised)
	if err := rs.ServeAsync(); err != nil {
		logger.Error("serve", "error", err.Error())
		os.Exit(1)
	}
	defer rs.Stop()

	done := make(chan float64, 1)
	sys.MustSpawn("coordinator", federated.NewCoordinatorProps(expected, rounds, initial, test, done))

	acc := <-done
	logger.Info("training finished", "final_accuracy", acc)
	sys.Shutdown(context.Background())
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
