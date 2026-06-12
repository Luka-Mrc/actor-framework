package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/lukam/actor-framework/federated"
	"github.com/lukam/actor-framework/federated/data"
	"github.com/lukam/actor-framework/federated/model"
	"github.com/lukam/actor-framework/framework"
	"github.com/lukam/actor-framework/framework/remote"
)

func main() {
	listen := env("LISTEN", ":9000")
	advertised := env("ADVERTISED", "peer:9000")
	id := env("NODE_ID", "peer")
	peers := splitCSV(env("PEERS", ""))
	numPeers := envInt("NUM_PEERS", len(peers)+1)
	shard := envInt("SHARD", 0)
	distribution := env("DISTRIBUTION", "iid")
	seed := int64(envInt("SEED", 42))
	epochs := envInt("EPOCHS", 1)
	lr := envFloat("LR", 0.01)
	rounds := envInt("TOTAL_ROUNDS", 5)
	dataDir := env("DATA_DIR", "/data")

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

	var shards []*data.Dataset
	if distribution == "noniid" {
		shards = data.SplitNonIID(train, numPeers, 0.8, seed)
	} else {
		shards = data.SplitIID(train, numPeers, seed)
	}
	if shard < 0 || shard >= len(shards) {
		logger.Error("bad shard index", "shard", shard, "num_peers", numPeers)
		os.Exit(1)
	}
	local := shards[shard]

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

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	sys.Shutdown(context.Background())
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

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
