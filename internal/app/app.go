package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lukam/actor-framework/federated"
	"github.com/lukam/actor-framework/federated/data"
	"github.com/lukam/actor-framework/federated/model"
	"github.com/lukam/actor-framework/framework"
	"github.com/lukam/actor-framework/framework/remote"
)

func newLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func RunCoordinator() {
	listen := Env("LISTEN", ":9000")
	advertised := Env("ADVERTISED", "coordinator:9000")
	expected := envInt("EXPECTED_TRAINERS", 3)
	rounds := envInt("TOTAL_ROUNDS", 5)
	roundTimeoutMs := envInt("ROUND_TIMEOUT_MS", 30000)
	dataDir := Env("DATA_DIR", "/data")
	seed := int64(envInt("SEED", 42))

	logger := newLogger()

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
		"expected_trainers", expected, "rounds", rounds, "round_timeout_ms", roundTimeoutMs,
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
	sys.MustSpawn("coordinator", federated.NewCoordinatorProps(
		expected, rounds, initial, test, time.Duration(roundTimeoutMs)*time.Millisecond, done))

	acc := <-done
	logger.Info("training finished", "final_accuracy", acc)
	sys.Shutdown(context.Background())
}

func RunTrainer() {
	listen := Env("LISTEN", ":9000")
	advertised := Env("ADVERTISED", "trainer:9000")
	id := Env("TRAINER_ID", "trainer")
	coordAddr := Env("COORDINATOR_ADDR", "actor://coordinator:9000/coordinator")
	numTrainers := envInt("NUM_TRAINERS", 3)
	shard := envInt("SHARD", 0)
	distribution := Env("DISTRIBUTION", "iid")
	seed := int64(envInt("SEED", 42))
	epochs := envInt("EPOCHS", 1)
	lr := envFloat("LR", 0.01)
	dataDir := Env("DATA_DIR", "/data")

	logger := newLogger()

	trainPath := filepath.Join(dataDir, "KDDTrain+.txt")
	train, err := data.LoadTrain(trainPath)
	if err != nil {
		logger.Error("load train", "path", trainPath, "error", err.Error())
		os.Exit(1)
	}
	local := shardOf(train, distribution, numTrainers, shard, seed, logger)

	sys := framework.NewActorSystem(id,
		framework.WithLogger(logger),
		framework.WithAdvertisedHost(advertised))
	rs := remote.NewRemoteSystem(sys, listen)
	rs.SetAdvertisedAddress(advertised)
	if err := rs.ServeAsync(); err != nil {
		logger.Error("serve", "error", err.Error())
		os.Exit(1)
	}
	defer rs.Stop()

	sys.MustSpawn(id, federated.NewTrainerProps(id, coordAddr, local, epochs, lr))
	logger.Info("trainer up",
		"id", id, "listen", listen, "advertised", advertised,
		"coordinator", coordAddr, "distribution", distribution,
		"shard", shard, "shard_size", local.Len(), "input_dim", local.InputDim())

	waitForSignal()
	sys.Shutdown(context.Background())
}

func RunPeer() {
	listen := Env("LISTEN", ":9000")
	advertised := Env("ADVERTISED", "peer:9000")
	id := Env("NODE_ID", "peer")
	peers := splitCSV(Env("PEERS", ""))
	numPeers := envInt("NUM_PEERS", len(peers)+1)
	shard := envInt("SHARD", 0)
	distribution := Env("DISTRIBUTION", "iid")
	seed := int64(envInt("SEED", 42))
	epochs := envInt("EPOCHS", 1)
	lr := envFloat("LR", 0.01)
	rounds := envInt("TOTAL_ROUNDS", 5)
	dataDir := Env("DATA_DIR", "/data")

	logger := newLogger()

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
	local := shardOf(train, distribution, numPeers, shard, seed, logger)
	initial := model.NewWeights(train.InputDim(), 64, 32, data.NumClasses, seed)

	sys := framework.NewActorSystem(id,
		framework.WithLogger(logger),
		framework.WithAdvertisedHost(advertised))
	rs := remote.NewRemoteSystem(sys, listen)
	rs.SetAdvertisedAddress(advertised)
	if err := rs.ServeAsync(); err != nil {
		logger.Error("serve", "error", err.Error())
		os.Exit(1)
	}
	defer rs.Stop()

	sys.MustSpawn(id, federated.NewPeerCoordinatorProps(id, peers, local, test, epochs, lr, rounds, initial, nil))
	logger.Info("peer up",
		"id", id, "listen", listen, "advertised", advertised,
		"peers", peers, "num_peers", numPeers, "distribution", distribution,
		"shard", shard, "shard_size", local.Len(), "input_dim", local.InputDim())

	waitForSignal()
	sys.Shutdown(context.Background())
}

func shardOf(train *data.Dataset, distribution string, n, shard int, seed int64, logger *slog.Logger) *data.Dataset {
	var shards []*data.Dataset
	if distribution == "noniid" {
		shards = data.SplitNonIID(train, n, 0.8, seed)
	} else {
		shards = data.SplitIID(train, n, seed)
	}
	if shard < 0 || shard >= len(shards) {
		logger.Error("bad shard index", "shard", shard, "n", n)
		os.Exit(1)
	}
	return shards[shard]
}

func waitForSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func Env(key, def string) string {
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

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
