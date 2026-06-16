# Specifikacija Projekta

## 1. Tim

Luka Marić RA154/2022
## 2. Zadatak

Za ciljnu ocenu 10 projekat obuhvata:

- Implementaciju generičkog aktorskog radnog okvira u programskom jeziku Go, sa osnovnim i dodatnim funkcionalnostima (supervizija aktora, middleware)
- Implementaciju sistema za federativno učenje nad aktorskim okvirom, primenjenog na problem detekcije mrežnih napada (intrusion detection), čime se demonstrira očuvanje privatnosti mrežnog saobraćaja između organizacija
- Podršku za dva režima rada: **provider klaster** (centralizovani koordinator) i **peer-to-peer** (decentralizovana agregacija), sa izborom prilikom pokretanja
- Upotrebu **CRDT** struktura za konzistentno praćenje stanja sistema u distribuiranom okruženju

## 3. Federativno učenje

### 3.1 Problem koji se rešava

Više organizacija (npr. kompanija, ISP-ova, data centara) želi da trenira zajednički model za klasifikaciju mrežnog saobraćaja na normalan i različite tipove napada (DoS, Probe, R2L, U2R). Nijedna organizacija ne želi da deli svoj sirovi mrežni saobraćaj sa ostalima jer on otkriva internu topologiju mreže, ranjivosti, i poverljive informacije o korisnicima. Federativno učenje omogućava treniranje zajedničkog modela bez centralizacije podataka. Svaka organizacija trenira lokalno i deli samo ažuriranja modela (gradijente/težine).

### 3.2 Algoritam

**Federated Averaging (FedAvg)** - McMahan et al., 2017.

Postupak po rundi:

1. Koordinator (ili peer inicijator) šalje trenutni globalni model svim učesnicima
2. Svaki učesnik trenira model lokalno na svojim podacima kroz E lokalnih epoha
3. Učesnik šalje ažurirane težine modela Aggregatoru (provider režim) ili svim susedima (P2P režim)
4. Aggregator primenjuje ponderisani prosek (provider režim), odnosno svaki peer lokalno agregira primljene težine i konvergira ka globalnom modelu (P2P režim)
5. Ponavljanje za R rundi

**Model:** Višeslojna potpuno povezana neuronska mreža (MLP):

- Ulazni sloj: ~122 dimenzije (41 NSL-KDD feature; 3 kategorijska — `protocol_type`, `service`, `flag` — se one-hot enkodiraju, pa je dimenzija veća od 41)
- Skriveni sloj 1: 64 neurona, ReLU aktivacija
- Skriveni sloj 2: 32 neurona, ReLU aktivacija
- Izlazni sloj: 5 klasa (Normal, DoS, Probe, R2L, U2R), softmax

### 3.3 Skup podataka

**NSL-KDD**

- Trening set: ~125.000 zapisa
- Test set: ~22.500 zapisa
- 41 numerički i kategorički feature
- 5 klasa: Normal, DoS, Probe, R2L, U2R

### 3.4 Distribucija podataka

- **IID**: nasumična jednolika raspodela zapisa među učesnicima
- **Non-IID**: svaki učesnik ima dominantnu klasu saobraćaja (simulira specijalizaciju organizacija)

### 3.5 Evaluacija rezultata

- **Metrike**: Accuracy, Precision/Recall/F1 po klasi, Confusion matrix
- **Konvergencija**: grafik gubitka i tačnosti po rundama za oba režima

## 4. Aktorski sistem

### 4.1 Vrste aktora i uloge

| Aktor | Uloga | Režim |
|---|---|---|
| **Coordinator** | Upravlja rundama, distribuira globalni model, prima rezultate agregacije, pokreće evaluaciju nakon svake runde | Provider |
| **Aggregator** | Prima težine od Trainera, prati ko je poslao update za tekuću rundu, računa ponderisani prosek, vraća novi globalni model Coordinatoru | Provider |
| **Trainer** | Učitava lokalne podatke, trenira model lokalno, šalje ažurirane težine Aggregatoru; čuva poslednju završenu rundu i težine radi idempotentnosti | Oba |
| **Evaluator** | Prima globalni model i test set, računa metrike, prijavljuje rezultate | Oba |
| **Logger** | Prima log poruke od svih aktora, formatira i zapisuje u strukturirani log | Oba |
| **PeerCoordinator** | Pokreće runde, prikuplja težine od suseda, lokalno agregira, propagira rezultat; sinhronizuje CRDT stanje sa ostalim peer-ovima | P2P |

> U provider režimu aktori su: Coordinator, Aggregator, Trainer, Evaluator, Logger.
> U P2P režimu: svaki čvor ima PeerCoordinator + Trainer + Evaluator + Logger (nema centralnog Aggregatora).

**Hijerarhija i pokretanje:**
- Coordinator pri pokretanju kreira Aggregator i Evaluator kao svoju decu (i postaje njihov supervisor).
- Traineri se pokreću zasebno (različiti procesi/mašine); adresa Coordinatora se prosleđuje kao konfiguracija (env `COORDINATOR_ADDR`, oblik `actor://host:port/coordinator`).
- Logger se kreira pri pokretanju i supervizuje se od strane Coordinatora

### 4.2 Poruke

| Poruka | Pošiljalac → Primalac | Potencijalni Sadržaj | Namena |
|---|---|---|---|
| `RegisterTrainer` | Trainer → Coordinator/PeerCoordinator | `{trainer_id, dataset_size, address}` | Registracija učesnika na početku; Coordinator pokreće prvu rundu kada se registruje očekivani broj Trainera |
| `StartRound` | Coordinator/PeerCoordinator → Trainer | `{round_number, global_weights [][]float64, aggregator_address}` | Početak nove runde, distribucija modela i adrese Aggregatora |
| `LocalUpdate` | Trainer → Aggregator/PeerCoordinator | `{trainer_id, round_number, updated_weights [][]float64, dataset_size, local_loss}` | Slanje lokalnih ažuriranja nakon treninga |
| `AggregationComplete` | Aggregator/PeerCoordinator → Coordinator/Trainer | `{round_number, new_global_weights [][]float64}` | Rezultat agregacije; Coordinator čuva novi globalni model u svom stanju |
| `EvaluateModel` | Coordinator/PeerCoordinator → Evaluator | `{round_number, weights [][]float64, test_data_path}` | Zahtev za evaluaciju modela koji se šalje nakon svake runde |
| `EvaluationResult` | Evaluator → Coordinator/PeerCoordinator | `{round_number, accuracy, class_metrics map[string]F1Score, confusion_matrix}` | Rezultati evaluacije |
| `TrainingComplete` | Coordinator/PeerCoordinator → Logger | `{total_rounds, final_accuracy, duration}` | Obaveštenje o završetku celokupnog treninga |
| `LogEntry` | * → Logger | `{timestamp, source_actor, level, message}` | Strukturirano logovanje događaja (fire-and-forget) |
| `SupervisionAlert` | ActorFramework (Aggregator/Evaluator) → Coordinator | `{failed_actor_id, error}` | Obaveštenje o padu child aktora; Coordinator odlučuje o akciji na osnovu sopstvenog stanja |
| `PeerSync` | PeerCoordinator → PeerCoordinator | `{peer_id, round_number, crdt_state, local_weights [][]float64, dataset_size}` | Sinhronizacija CRDT stanja između peer-ova |

### 4.3 Napomene o pouzdanosti

**Timeout mehanizam:** Coordinator pokreće tajmer pri slanju `StartRound`. Ako u roku od X sekundi ne stigne `LocalUpdate` od nekog Trainera, taj Trainer se otpisuje za tekuću rundu i agregacija se nastavlja sa primljenim ažuriranjima. Ovo obezbeđuje da spori ili pali Traineri ne blokiraju sistem.

**Idempotentnost Trainera:** Trainer čuva `last_completed_round` i `last_sent_weights` u internom stanju. Ako primi `StartRound` sa istim `round_number` koji je već obradio (npr. zbog restarta Aggregatora), Trainer ponovo šalje iste težine bez ponovnog treniranja. Stare vrednosti se brišu iz memorije kada stigne nova runda.

**Pad Aggregatora usred runde:** framework (supervizija roditelja) restartuje Aggregator, a Coordinator dobija `SupervisionAlert` i ponovo šalje `StartRound` sa istim `round_number` svim Trainerima. Zahvaljujući idempotentnosti, Traineri samo prosleđuju već izračunate težine, pa sveže pokrenuti Aggregator ponovo prikupi ažuriranja.

**Pad Evaluatora između rundi:** framework restartuje Evaluator; Coordinator dobija `SupervisionAlert` i samo to zabeleži (bez dodatnih akcija).

### 4.4 Skica komunikacije

```
=== PROVIDER REŽIM (zvezdasta topologija) ===

                        ┌────────┐
                        │ Logger │
                        └───▲────┘
                            │ LogEntry (svi aktori)
    ┌───────────────────────┴──────────────────────┐
    │                  Coordinator                  │
    │        (upravlja rundama, supervizija)        │
    └───┬──────────────────────────────┬────────────┘
        │                              │               ▲
        │ StartRound                   │ EvaluateModel │ EvaluationResult
        │ (global_weights,             │               │
        │  aggregator_address)         ▼               │
        ▼                         ┌───────────┐────────┘
   ┌────────┐  ┌────────┐         │ Evaluator │
   │Trainer1│  │Trainer2│         └───────────┘
   │(Org A) │  │(Org B) │
   └────┬───┘  └────┬───┘
        │           │ LocalUpdate
        └─────┬─────┘  {trainer_id, round_number,
              │         updated_weights, dataset_size}
              ▼
        ┌──────────┐
        │Aggregator│ (prati ko je poslao, ponderisani prosek)
        └─────┬────┘
              │ AggregationComplete
              ▼
         Coordinator (čuva novi globalni model)


=== PEER-TO-PEER REŽIM (mesh topologija) ===

 ┌──────────────────┐     PeerSync      ┌──────────────────┐
 │   Čvor A (Org A) │◄────────────────►│   Čvor B (Org B) │
 │ ┌──────────────┐ │                   │ ┌──────────────┐ │
 │ │PeerCoordinator│ │                   │ │PeerCoordinator│ │
 │ │  + Trainer    │ │     PeerSync      │ │  + Trainer    │ │
 │ │  + Evaluator  │ │◄────────────────►│ │  + Evaluator  │ │
 │ │  + Logger     │ │                   │ │  + Logger     │ │
 │ └──────────────┘ │                   │ └──────────────┘ │
 └──────────────────┘                   └──────────────────┘
            ▲                PeerSync             ▲
            └──────────┐              ┌───────────┘
                       ▼              ▼
                 ┌──────────────────┐
                 │   Čvor C (Org C) │
                 │ ┌──────────────┐ │
                 │ │PeerCoordinator│ │
                 │ │  + Trainer    │ │
                 │ │  + Evaluator  │ │
                 │ │  + Logger     │ │
                 │ └──────────────┘ │
                 └──────────────────┘

CRDT stanje (G-Counter, OR-Set) se sinhronizuje
kroz PeerSync poruke pri svakoj komunikaciji.
```

### 4.5 CRDT upotreba

Za konzistentno praćenje distribuiranog stanja bez centralnog autoriteta koriste se sledeće CRDT strukture:

- **G-Counter** (Grow-only Counter): svaki peer vodi lokalni brojač završenih rundi. Merge operacija uzima max po čvoru. Koristi se za detekciju kada su svi peer-ovi završili tekuću rundu i za koordinacija bez centralnog koordinatora.
- **OR-Set** (Observed-Remove Set): praćenje skupa aktivnih učesnika (registracija/deregistracija). Omogućava konzistentnu sliku učesnika čak i pri konkurentnim join/leave operacijama u P2P režimu.

U provider režimu OR-Set se koristi za praćenje skupa registrovanih Trainera, što omogućava konzistentnu sliku učesnika pri konkurentnim join/leave operacijama.

## 5. Aktorski radni okvir

- **Obavezni elementi**: Aktori, asinhrone poruke, Mailbox, Become, Lifecycle, Remote (gRPC)
- **Dodatni elementi**: Supervizija (restart strategija za pale aktore), Middleware (logovanje poruka, merenje latencije)

Objedinjeni ulaz `cmd/node` bira režim pri pokretanju: `--mode=provider --role=coordinator|trainer` ili `--mode=p2p` (uloga je uvek peer). Alternativno postoje i zasebni binari `cmd/coordinator`, `cmd/trainer`, `cmd/peer`. Demonstracija na više mašina putem gRPC remote aktora.

## 6. Tehnologije

Go 1.25+, gRPC + Protocol Buffers, Docker, `log/slog`. Neuronska mreža (MLP + backpropagation) i parsiranje NSL-KDD dataseta implementirani su ručno, bez eksternih ML ili CSV biblioteka.

## 7. Pokretanje

### 7.1 Preduslovi

- **Go 1.25+** (isti major kao u `go.mod`; Docker koristi `golang:1.25`).
- **Dataset** je uključen u repozitorijum: `federated/data/KDDTrain+.txt` i `KDDTest+.txt` (NSL-KDD). Ne mora se skidati.
- Za regenerisanje `.pb.go` fajlova iz `.proto` treba `protoc` + `protoc-gen-go`/`protoc-gen-go-grpc`; **nije potrebno za pokretanje** jer su generisani fajlovi u repozitorijumu.

### 7.2 Provera build-a

```
go build ./...
go vet ./...
```

Postoje četiri izvršna programa: `cmd/coordinator` i `cmd/trainer` (provider režim), `cmd/peer` (P2P režim), i `cmd/node` — objedinjeni ulaz koji režim bira flag-om `--mode`/`--role` (poziva istu logiku). Svi se konfigurišu preko env promenljivih.

Objedinjeni ulaz (npr. umesto `go run ./cmd/coordinator`):

```
go run ./cmd/node --mode=provider --role=coordinator
go run ./cmd/node --mode=provider --role=trainer
go run ./cmd/node --mode=p2p
```

### 7.3 Docker (najlakše)

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

Izbor režima pri pokretanju = koji compose fajl se pokrene. Dataset se u oba slučaja montira kao read-only volume iz `federated/data`, ne ubacuje se u image.

### 7.4 Ručno, bez Dockera (localhost)

Svaki proces se pokreće u zasebnom terminalu, sa svojim portom. Na jednoj mašini `ADVERTISED` je `127.0.0.1:<port>`.

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

Za trainer-2/3: port `:9002`/`:9003`, `ADVERTISED` odgovarajući, `TRAINER_ID`=`trainer-2`/`trainer-3`, `SHARD`=`1`/`2`.

**P2P — 3 terminala.** Peer-1 (peer-2/3 analogno, uz izmenu porta, `NODE_ID`, `SHARD` i `PEERS`):

```powershell
$env:LISTEN=":9101"; $env:ADVERTISED="127.0.0.1:9101"
$env:NODE_ID="peer-1"; $env:SHARD="0"
$env:PEERS="actor://127.0.0.1:9102/peer-2,actor://127.0.0.1:9103/peer-3"
$env:NUM_PEERS="3"; $env:DISTRIBUTION="iid"
$env:SEED="42"; $env:EPOCHS="1"; $env:LR="0.01"; $env:TOTAL_ROUNDS="5"; $env:DATA_DIR="federated/data"
go run ./cmd/peer
```

### 7.5 Env promenljive

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

### 7.6 Šta treba znati

- **Adresa aktora** je `actor://<ADVERTISED>/<ime>`, gde je ime koordinatora uvek `coordinator`, trenera = `TRAINER_ID`, peera = `NODE_ID`. Zato `COORDINATOR_ADDR` i `PEERS` imaju taj oblik.
- **Portovi:** na jednoj mašini svaki proces mora imati različit port. Podrazumevane vrednosti su za Docker, pa na localhostu obavezno pregazi `ADVERTISED` i port.
- **Doslednost šardova:** `SEED`, `NUM_TRAINERS`/`NUM_PEERS` i `DISTRIBUTION` moraju biti isti kod svih, a `SHARD` jedinstven.
- **Redosled pokretanja nije bitan** — treneri i peer-ovi imaju retry petlju za registraciju/sinhronizaciju.
- **Kraj:** coordinator sam izađe po završetku; treneri i peer-ovi rade dok se ne prekinu sa Ctrl+C.
- **Više mašina:** `ADVERTISED` postavi na LAN IP mašine, a `COORDINATOR_ADDR`/`PEERS` koriste te IP adrese; otvori portove u firewall-u.
