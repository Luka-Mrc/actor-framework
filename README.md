# actor-framework

Generički aktorski radni okvir u Go-u, sa sistemom za **federativno učenje** izgrađenim nad njim (detekcija mrežnih napada na NSL-KDD skupu). Podržava **provider** (centralizovani koordinator) i **peer-to-peer** (decentralizovana agregacija) režim, uz **CRDT** za konzistentno distribuirano stanje.

## Dokumentacija

- **Specifikacija projekta:** [`dokumentacija/SpecifikacijaProjekta.pdf`](dokumentacija/SpecifikacijaProjekta.pdf) (izvor: [`SpecifikacijaProjekta.md`](dokumentacija/SpecifikacijaProjekta.md)) — problem, algoritam, aktori, poruke, skica komunikacije, CRDT.
- **Plan izrade po fazama:** [`dokumentacija/PLAN.md`](dokumentacija/PLAN.md).

## Pregled

- `framework/` — generički okvir: aktori, asinhrone poruke, mailbox, `Become`, lifecycle (PreStart/PostStop), supervizija, middleware, remote (gRPC).
- `federated/` — FL nad okvirom: MLP + backprop (ručno), NSL-KDD pipeline, FedAvg, aktori (Coordinator, Aggregator, Trainer, Evaluator, Logger, PeerCoordinator).
- `crdt/` — G-Counter i OR-Set (generičke CvRDT strukture).
- `cmd/` — izvršni programi (`coordinator`, `trainer`, `peer`, i objedinjeni `node`).

## Struktura repozitorijuma

| Direktorijum | Sadržaj |
|---|---|
| `framework/` | jezgro aktorskog okvira |
| `framework/remote/` | gRPC transport (Envelope, codec) |
| `federated/` | FL aktori, FedAvg, poruke |
| `federated/data`, `federated/model` | NSL-KDD pipeline, MLP |
| `crdt/` | CRDT strukture |
| `cmd/` | main-ovi (binari) |
| `internal/app/` | zajednička logika pokretanja |
| `dokumentacija/` | specifikacija + plan |

## Preduslovi

- **Go 1.25+** (isti major kao u `go.mod`; Docker koristi `golang:1.25`).
- **Dataset** je uključen u repozitorijum: `federated/data/KDDTrain+.txt` i `KDDTest+.txt` (NSL-KDD).
- Za regenerisanje `.pb.go` iz `.proto` treba `protoc` + `protoc-gen-go`/`protoc-gen-go-grpc`; **nije potrebno za pokretanje** jer su generisani fajlovi u repozitorijumu.

## Provera build-a

```
go build ./...
go vet ./...
```

## Pokretanje — Docker (najlakše)

Provider režim (1 coordinator + 3 trainer kontejnera):

```
docker compose up --build --abort-on-container-exit
```

Kad coordinator ispiše `training finished`, `--abort-on-container-exit` gasi i ostale kontejnere.

P2P režim (3 ravnopravna peer čvora, bez koordinatora):

```
docker compose -f docker-compose.p2p.yml up --build
```

Peer čvorovi ne izlaze sami; kad svi ispišu `peer training complete`, prekini sa Ctrl+C.

Izbor režima = koji compose fajl se pokrene. Dataset se montira kao read-only volume iz `federated/data`.

## Pokretanje — ručno, bez Dockera (localhost)

Svaki proces u zasebnom terminalu, sa svojim portom. Na jednoj mašini `ADVERTISED` je `127.0.0.1:<port>`.

**Provider — 4 terminala.** Terminal 1 (coordinator):

```powershell
$env:LISTEN=":9000"; $env:ADVERTISED="127.0.0.1:9000"
$env:EXPECTED_TRAINERS="3"; $env:TOTAL_ROUNDS="5"; $env:SEED="42"
$env:DATA_DIR="federated/data"
go run ./cmd/coordinator
```

Terminali 2–4 (trainer-1/2/3) — razlikuju se samo u portu, `ADVERTISED`, `TRAINER_ID` i `SHARD`:

```powershell
$env:LISTEN=":9001"; $env:ADVERTISED="127.0.0.1:9001"
$env:TRAINER_ID="trainer-1"; $env:SHARD="0"
$env:COORDINATOR_ADDR="actor://127.0.0.1:9000/coordinator"
$env:NUM_TRAINERS="3"; $env:DISTRIBUTION="iid"
$env:SEED="42"; $env:EPOCHS="1"; $env:LR="0.01"; $env:DATA_DIR="federated/data"
go run ./cmd/trainer
```

Za trainer-2/3: port `:9002`/`:9003`, odgovarajući `ADVERTISED`, `TRAINER_ID`=`trainer-2`/`trainer-3`, `SHARD`=`1`/`2`.

**P2P — 3 terminala.** Peer-1 (peer-2/3 analogno, uz izmenu porta, `NODE_ID`, `SHARD` i `PEERS`):

```powershell
$env:LISTEN=":9101"; $env:ADVERTISED="127.0.0.1:9101"
$env:NODE_ID="peer-1"; $env:SHARD="0"
$env:PEERS="actor://127.0.0.1:9102/peer-2,actor://127.0.0.1:9103/peer-3"
$env:NUM_PEERS="3"; $env:DISTRIBUTION="iid"
$env:SEED="42"; $env:EPOCHS="1"; $env:LR="0.01"; $env:TOTAL_ROUNDS="5"; $env:DATA_DIR="federated/data"
go run ./cmd/peer
```

**Objedinjeni ulaz** (`cmd/node`) bira režim flag-om (poziva istu logiku):

```
go run ./cmd/node --mode=provider --role=coordinator
go run ./cmd/node --mode=provider --role=trainer
go run ./cmd/node --mode=p2p
```

## Env promenljive

| Promenljiva | Programi | Značenje |
|---|---|---|
| `LISTEN` | svi | lokalni bind, npr. `:9000` |
| `ADVERTISED` | svi | adresa kojom drugi dosežu ovaj proces (`host:port`); na više mašina = LAN IP mašine |
| `DATA_DIR` | svi | folder sa `KDDTrain+.txt`/`KDDTest+.txt` |
| `SEED` | svi | seme za deljenje šardova i init modela (isto kod svih) |
| `DISTRIBUTION` | trainer, peer | `iid` ili `noniid` |
| `EPOCHS`, `LR` | trainer, peer | lokalne epohe i learning rate |
| `SHARD` | trainer, peer | indeks šarda, `0..N-1`, jedinstven po čvoru |
| `EXPECTED_TRAINERS`, `TOTAL_ROUNDS` | coordinator | broj trenera pre starta / broj rundi |
| `ROUND_TIMEOUT_MS` | coordinator | rok za rundu; po isteku se agregira sa primljenim update-ima (default 30000) |
| `TRAINER_ID`, `COORDINATOR_ADDR`, `NUM_TRAINERS` | trainer | id trenera / adresa koordinatora / ukupno trenera |
| `NODE_ID`, `PEERS`, `NUM_PEERS`, `TOTAL_ROUNDS` | peer | id čvora / adrese ostalih peer-ova (CSV) / ukupno čvorova / broj rundi |

## Šta treba znati

- **Adresa aktora** je `actor://<ADVERTISED>/<ime>` (ime koordinatora je `coordinator`, trenera = `TRAINER_ID`, peera = `NODE_ID`). Zato `COORDINATOR_ADDR` i `PEERS` imaju taj oblik.
- **Portovi:** na jednoj mašini svaki proces mora imati različit port. Podrazumevane vrednosti su za Docker, pa na localhostu pregazi `ADVERTISED` i port.
- **Doslednost šardova:** `SEED`, `NUM_TRAINERS`/`NUM_PEERS` i `DISTRIBUTION` isti kod svih; `SHARD` jedinstven.
- **Redosled pokretanja nije bitan** — treneri i peer-ovi imaju retry petlju.
- **Kraj:** coordinator sam izađe; treneri i peer-ovi rade dok se ne prekinu sa Ctrl+C.
- **Više mašina:** `ADVERTISED` = LAN IP mašine; `COORDINATOR_ADDR`/`PEERS` koriste te IP adrese; otvori portove u firewall-u.

## Regenerisanje specifikacije (PDF)

```
powershell -File scripts/build-spec-pdf.ps1
```

Konvertuje `dokumentacija/SpecifikacijaProjekta.md` u PDF (Python + Microsoft Edge headless, bez eksternih zavisnosti).
