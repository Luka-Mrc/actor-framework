// Command trainer pokrece jedan Trainer kao zaseban proces, ucitava svoj sard
// iz zajednickog train skupa i registruje se kod koordinatora preko gRPC-a.
// Konfiguracija je env-driven (vidi docker-compose.yml).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/lukam/actor-framework/federated"
	"github.com/lukam/actor-framework/federated/data"
	"github.com/lukam/actor-framework/framework"
	"github.com/lukam/actor-framework/framework/remote"
)

func main() {
	listen := env("LISTEN", ":9000")
	advertised := env("ADVERTISED", "trainer:9000")
	id := env("TRAINER_ID", "trainer")
	coordAddr := env("COORDINATOR_ADDR", "actor://coordinator:9000/coordinator")
	numTrainers := envInt("NUM_TRAINERS", 3)
	shard := envInt("SHARD", 0)
	distribution := env("DISTRIBUTION", "iid")
	seed := int64(envInt("SEED", 42))
	epochs := envInt("EPOCHS", 1)
	lr := envFloat("LR", 0.01)
	dataDir := env("DATA_DIR", "/data")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	trainPath := filepath.Join(dataDir, "KDDTrain+.txt")
	train, err := data.LoadTrain(trainPath)
	if err != nil {
		logger.Error("load train", "path", trainPath, "error", err.Error())
		os.Exit(1)
	}

	var shards []*data.Dataset
	if distribution == "noniid" {
		shards = data.SplitNonIID(train, numTrainers, 0.8, seed)
	} else {
		shards = data.SplitIID(train, numTrainers, seed)
	}
	if shard < 0 || shard >= len(shards) {
		logger.Error("bad shard index", "shard", shard, "num_trainers", numTrainers)
		os.Exit(1)
	}
	local := shards[shard]

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

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
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

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
