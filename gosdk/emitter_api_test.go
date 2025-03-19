package gosdk

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/require"
	"log"
	"math/rand"
	"net"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"pgregory.net/rapid"

	emitterproto "github.com/0xAtelerix/sdk/proto"
	"github.com/0xAtelerix/sdk/types"

	"google.golang.org/grpc"
)

func TestEmitterCall(t *testing.T) {
	dbPath := "./testdb"
	_ = os.RemoveAll(dbPath)   // Очищаем базу перед тестом
	defer os.RemoveAll(dbPath) // Очищаем базу после теста

	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(dbPath).
		WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
			return kv.TableCfg{
				checkpointBucket: {},
				externalTxBucket: {},
				blocksBucket:     {},
			}
		}).
		Open()

	// Создаем сервер с MDBX
	srv := NewServer(db, 1)
	if err != nil {
		t.Fatalf("Ошибка создания сервера: %v", err)
	}
	defer srv.appchainDB.Close()

	// Записываем тестовые чекпоинты через WriteCheckpoint
	ctx := context.Background()

	checkpoints := []types.Checkpoint{
		{
			ChainID:                  1,
			BlockNumber:              100,
			BlockHash:                [32]byte{1},
			StateRoot:                [32]byte{11},
			ExternalTransactionsRoot: [32]byte{111},
		},
		{
			ChainID:                  1,
			BlockNumber:              200,
			BlockHash:                [32]byte{2},
			StateRoot:                [32]byte{22},
			ExternalTransactionsRoot: [32]byte{222},
		},
	}

	tx, err := db.BeginRw(context.TODO())
	if err != nil {
		t.Fatalf("Ошибка запуска сервера: %v", err)
	}

	for _, chk := range checkpoints {
		if err := WriteCheckpoint(ctx, tx, chk); err != nil {
			t.Fatalf("Ошибка записи чекпоинта: %v", err)
		}
	}
	err = tx.Commit()
	require.NoError(t, err)
	// Запускаем gRPC сервер
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		listener, err := net.Listen("tcp", ":50051")
		if err != nil {
			t.Fatalf("Ошибка запуска сервера: %v", err)
		}

		grpcServer := grpc.NewServer()
		emitterproto.RegisterEmitterServer(grpcServer, srv)

		log.Println("Сервер слушает на порту 50051...")
		wg.Done()
		if err := grpcServer.Serve(listener); err != nil {
			t.Fatalf("Ошибка gRPC сервера: %v", err)
		}
	}()
	wg.Wait()
	time.Sleep(time.Millisecond * 100)

	// Подключение к серверу
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Не удалось подключиться: %v", err)
	}
	defer conn.Close()

	client := emitterproto.NewEmitterClient(conn)

	// Отправка запроса
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	limit := uint32(5)
	req := &emitterproto.GetCheckpointsRequest{
		LatestPreviousCheckpointBlockNumber: 50, // Ожидаем все чекпоинты от 50 и выше
		Limit:                               &limit,
	}

	res, err := client.GetCheckpoints(ctx, req)
	if err != nil {
		t.Fatalf("Ошибка вызова GetCheckpoints: %v", err)
	}

	// Проверяем чекпоинты
	if len(res.Checkpoints) != len(checkpoints) {
		t.Fatalf("Ожидалось %d чекпоинтов, получено %d", len(checkpoints), len(res.Checkpoints))
	}

	for _, checkpoint := range res.Checkpoints {
		log.Printf("Блок: %d, StateRoot: %x, BlockHash: %x, ExternalTxRoot: %x",
			checkpoint.LatestBlockNumber,
			checkpoint.StateRoot,
			checkpoint.BlockHash,
			checkpoint.ExternalTxRootHash)
	}
}

// Генерация случайного чекпоинта
func randomCheckpoint(t *rapid.T, chainID uint64, blockNumber uint64) types.Checkpoint {
	return types.Checkpoint{
		ChainID:                  chainID,
		BlockNumber:              blockNumber,
		StateRoot:                [32]byte(rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "StateRoot")),
		BlockHash:                [32]byte(rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "BlockHash")),
		ExternalTransactionsRoot: [32]byte(rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "ExternalTxRoot")),
	}
}

// Запускаем gRPC сервер в отдельной горутине
func startGRPCServer(t *rapid.T, srv *AppchainEmitterServer) (string, func()) {
	listener, err := net.Listen("tcp", ":0") // Используем ":0", чтобы ОС выделила свободный порт
	if err != nil {
		t.Fatalf("Ошибка запуска сервера: %v", err)
	}

	grpcServer := grpc.NewServer()
	emitterproto.RegisterEmitterServer(grpcServer, srv)

	addr := listener.Addr().String()
	//log.Printf("Сервер слушает на %s...", addr)

	stop := func() {
		grpcServer.Stop()
		listener.Close()
	}

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Ошибка gRPC сервера: %v", err)
		}
	}()

	time.Sleep(time.Millisecond * 100) // Даем серверу время запуститься
	return addr, stop
}

func TestEmitterCall_PropertyBased(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		dbPath := os.TempDir() + "/TestEmitterCall_PropertyBased" + strconv.Itoa(rand.Int())
		_ = os.RemoveAll(dbPath)   // Очищаем базу перед тестом
		defer os.RemoveAll(dbPath) // Удаляем базу после теста

		db, err := mdbx.NewMDBX(mdbxlog.New()).
			Path(dbPath).
			WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
				return kv.TableCfg{
					checkpointBucket: {},
					externalTxBucket: {},
					blocksBucket:     {},
				}
			}).
			Open()

		tx, err := db.BeginRw(context.TODO())
		if err != nil {
			t.Fatalf("DB: %v", err)
		}
		defer tx.Rollback()
		// Создаем сервер с MDBX
		srv := NewServer(db, 1)
		defer srv.appchainDB.Close()

		// Запускаем gRPC сервер и получаем динамический адрес
		addr, stopServer := startGRPCServer(t, srv)
		defer stopServer() // Гарантируем закрытие сервера после теста

		// Генерируем случайное количество чекпоинтов (до 100)
		numCheckpoints := rapid.IntRange(1, 100).Draw(t, "numCheckpoints")
		checkpointMap := make(map[uint64]types.Checkpoint)

		// Заполняем уникальные LatestBlockNumber
		for len(checkpointMap) < numCheckpoints {
			blockNumber := rapid.Uint64().Draw(t, "LatestBlockNumber")
			checkpointMap[blockNumber] = randomCheckpoint(t, 1, blockNumber)
		}

		// Преобразуем map в список и записываем в MDBX
		checkpoints := make([]types.Checkpoint, 0, len(checkpointMap))
		for _, chk := range checkpointMap {
			checkpoints = append(checkpoints, chk)
			if err := WriteCheckpoint(context.Background(), tx, chk); err != nil {
				t.Fatalf("Ошибка записи чекпоинта: %v", err)
			}
		}
		err = tx.Commit()
		require.NoError(t, err)
		// Сортируем чекпоинты по LatestBlockNumber
		sort.Slice(checkpoints, func(i, j int) bool {
			return checkpoints[i].BlockNumber < checkpoints[j].BlockNumber
		})

		// Подключение к серверу
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials())) // Используем динамический адрес
		if err != nil {
			t.Fatalf("Не удалось подключиться: %v", err)
		}
		defer conn.Close()

		client := emitterproto.NewEmitterClient(conn)

		// Генерируем случайный запрос
		startBlock := rapid.Uint64().Draw(t, "startBlock")
		limit := rapid.Uint32Range(1, uint32(len(checkpoints))).Draw(t, "limit")

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		req := &emitterproto.GetCheckpointsRequest{
			ChainId:                             1,
			LatestPreviousCheckpointBlockNumber: startBlock,
			Limit:                               &limit,
		}

		res, err := client.GetCheckpoints(ctx, req)
		if err != nil {
			t.Fatalf("Ошибка вызова GetCheckpoints: %v", err)
		}

		chainIDRes, err := client.GetChainId(ctx, &emptypb.Empty{})
		if err != nil {
			t.Fatalf("GetChainId: %v", err)
		}

		// ✅ Проверяем, что чекпоинты в ответе соответствуют условиям запроса
		expectedCheckpoints := make([]types.Checkpoint, 0)
		for _, chk := range checkpoints {
			if chk.BlockNumber >= startBlock {
				expectedCheckpoints = append(expectedCheckpoints, chk)
				if len(expectedCheckpoints) >= int(limit) {
					break
				}
			}
		}

		// ✅ Проверяем, что количество чекпоинтов соответствует лимиту
		if len(res.Checkpoints) != len(expectedCheckpoints) {
			t.Fatalf("Ошибка: ожидалось %d чекпоинтов, получено %d", len(expectedCheckpoints), len(res.Checkpoints))
		}

		// ✅ Проверяем, что чекпоинты отсортированы по LatestBlockNumber
		prevBlockNumber := uint64(0)
		for i, chk := range res.Checkpoints {
			if chk.LatestBlockNumber < startBlock {
				t.Fatalf("Ошибка: чекпоинт %d меньше стартового блока %d", chk.LatestBlockNumber, startBlock)
			}
			if chk.LatestBlockNumber < prevBlockNumber {
				t.Fatalf("Ошибка: чекпоинты не отсортированы по LatestBlockNumber")
			}
			prevBlockNumber = chk.LatestBlockNumber

			// ✅ Проверяем, что данные чекпоинта совпадают с ожидаемыми
			expected := expectedCheckpoints[i]
			if chainIDRes.ChainId != expected.ChainID ||
				!bytes.Equal(chk.StateRoot, expected.StateRoot[:]) ||
				!bytes.Equal(chk.BlockHash, expected.BlockHash[:]) ||
				!bytes.Equal(chk.ExternalTxRootHash, expected.ExternalTransactionsRoot[:]) {
				t.Fatalf("Ошибка: данные чекпоинта не совпадают с записанными")
			}
		}
	})
}

func TestGetExternalTransactions_PropertyBased(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		dbPath := os.TempDir() + "/TestGetExternalTransactions_PropertyBased" + strconv.Itoa(rand.Int())
		_ = os.RemoveAll(dbPath)
		defer os.RemoveAll(dbPath)

		db, err := mdbx.NewMDBX(mdbxlog.New()).
			Path(dbPath).
			WithTableCfg(func(defaultBuckets kv.TableCfg) kv.TableCfg {
				return kv.TableCfg{
					checkpointBucket: {},
					externalTxBucket: {},
					blocksBucket:     {},
				}
			}).
			Open()

		srv := NewServer(db, 1)
		defer srv.appchainDB.Close()

		tx, err := db.BeginRw(context.TODO())
		if err != nil {
			t.Fatalf("DB: %v", err)
		}

		addr, stopServer := startGRPCServer(t, srv)
		defer stopServer()

		numBlocks := rapid.IntRange(1, 50).Draw(t, "numBlocks")
		transactionMap := make(map[uint64][]types.ExternalTransaction)

		for i := 0; i < numBlocks; i++ {
			blockNumber := rapid.Uint64().Draw(t, "BlockNumber")
			numTx := rapid.IntRange(1, 20).Draw(t, "numTxInBlock")

			for j := 0; j < numTx; j++ {
				tx := types.ExternalTransaction{
					ChainID: rapid.Uint64().Draw(t, "ChainId"),
					Tx:      rapid.SliceOfN(rapid.Byte(), 64, 64).Draw(t, "Tx"),
				}
				transactionMap[blockNumber] = append(transactionMap[blockNumber], tx)
			}

			if _, err := WriteExternalTransactions(context.Background(), tx, blockNumber, transactionMap[blockNumber]); err != nil {
				t.Fatalf("Ошибка записи транзакции: %v", err)
			}
		}

		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("Не удалось подключиться: %v", err)
		}
		defer conn.Close()

		client := emitterproto.NewEmitterClient(conn)

		startBlock := rapid.Uint64().Draw(t, "startBlock")
		limit := rapid.Uint32Range(1, 100).Draw(t, "limit")

		req := &emitterproto.GetExternalTransactionsRequest{
			LatestPreviousBlockNumber: startBlock,
			Limit:                     &limit,
		}

		res, err := client.GetExternalTransactions(context.Background(), req)
		if err != nil {
			t.Fatalf("Ошибка вызова GetExternalTransactions: %v", err)
		}

		for _, blk := range res.Blocks {
			if blk.BlockNumber < startBlock {
				t.Fatalf("Ошибка: блок %d меньше startBlock %d", blk.BlockNumber, startBlock)
			}

			if txs, exists := transactionMap[blk.BlockNumber]; exists {
				if len(blk.ExternalTransactions) != len(txs) {
					t.Fatalf("Ошибка: несоответствие количества транзакций для блока %d", blk.BlockNumber)
				}
			} else {
				t.Fatalf("Ошибка: блок %d не найден в ожидаемых данных", blk.BlockNumber)
			}
		}
	})
}
