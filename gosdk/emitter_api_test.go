package gosdk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	mdbxlog "github.com/ledgerwatch/log/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"pgregory.net/rapid"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	emitterproto "github.com/0xAtelerix/sdk/gosdk/proto"
)

func TestEmitterCall(t *testing.T) {
	dbPath := t.TempDir()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	db, err := mdbx.NewMDBX(mdbxlog.New()).
		Path(dbPath).
		WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
			return kv.TableCfg{
				CheckpointBucket: {},
				ExternalTxBucket: {},
				BlocksBucket:     {},
			}
		}).
		Open()

	// Создаем сервер с MDBX
	srv := NewServer[*CustomTransaction[Receipt]](db, 1, nil)
	if err != nil {
		t.Fatalf("Ошибка создания сервера: %v", err)
	}
	defer srv.appchainDB.Close()

	// Записываем тестовые чекпоинты через WriteCheckpoint
	ctx := t.Context()

	checkpoints := []apptypes.Checkpoint{
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

	var tx kv.RwTx

	tx, err = db.BeginRw(t.Context())
	if err != nil {
		t.Fatalf("Ошибка запуска сервера: %v", err)
	}

	for _, chk := range checkpoints {
		if err = WriteCheckpoint(ctx, tx, chk); err != nil {
			t.Fatalf("Ошибка записи чекпоинта: %v", err)
		}
	}

	err = tx.Commit()
	require.NoError(t, err)

	// Запускаем gRPC сервер
	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		var listener net.Listener

		listener, err = (&net.ListenConfig{}).Listen(t.Context(), "tcp", ":50051")
		if err != nil {
			t.Errorf("Ошибка запуска сервера: %v", err)

			return
		}

		grpcServer := grpc.NewServer()
		emitterproto.RegisterEmitterServer(grpcServer, srv)

		log.Info().Msg("Сервер слушает на порту 50051...")

		wg.Done()

		if err = grpcServer.Serve(listener); err != nil {
			t.Errorf("Ошибка gRPC сервера: %v", err)

			return
		}
	}()

	wg.Wait()
	time.Sleep(time.Millisecond * 500)

	// Подключение к серверу
	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Не удалось подключиться: %v", err)
	}

	defer func() {
		connErr := conn.Close()
		require.NoError(t, connErr)
	}()

	client := emitterproto.NewEmitterClient(conn)

	// Отправка запроса
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	limit := uint32(5)
	req := &emitterproto.GetCheckpointsRequest{
		LatestPreviousCheckpointBlockNumber: 50, // Ожидаем все чекпоинты от 50 и выше
		Limit:                               &limit,
	}

	var res *emitterproto.CheckpointResponse

	res, err = client.GetCheckpoints(ctx, req)
	if err != nil {
		t.Fatalf("Ошибка вызова GetCheckpoints: %v", err)
	}

	// Проверяем чекпоинты
	if len(res.GetCheckpoints()) != len(checkpoints) {
		t.Fatalf(
			"Ожидалось %d чекпоинтов, получено %d",
			len(checkpoints),
			len(res.GetCheckpoints()),
		)
	}

	for _, checkpoint := range res.GetCheckpoints() {
		log.Printf("Блок: %d, StateRoot: %x, BlockHash: %x, ExternalTxRoot: %x",
			checkpoint.GetLatestBlockNumber(),
			checkpoint.GetStateRoot(),
			checkpoint.GetBlockHash(),
			checkpoint.GetExternalTxRootHash())
	}
}

// Генерация случайного чекпоинта
func randomCheckpoint(t *rapid.T, chainID uint64, blockNumber uint64) apptypes.Checkpoint {
	return apptypes.Checkpoint{
		ChainID:     chainID,
		BlockNumber: blockNumber,
		StateRoot: [32]byte(
			rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "StateRoot"),
		),
		BlockHash: [32]byte(
			rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "BlockHash"),
		),
		ExternalTransactionsRoot: [32]byte(
			rapid.SliceOfN(rapid.Byte(), 32, 32).Draw(t, "ExternalTxRoot"),
		),
	}
}

// Запускаем gRPC сервер в отдельной горутине
func startGRPCServer[apptx apptypes.AppTransaction[R], R apptypes.Receipt](
	t *rapid.T,
	srv *AppchainEmitterServer[apptx, R],
) (string, func()) {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(
		t.Context(),
		"tcp",
		":0",
	) // Используем ":0", чтобы ОС выделила свободный порт
	if err != nil {
		t.Fatalf("Ошибка запуска сервера: %v", err)
	}

	grpcServer := grpc.NewServer()
	emitterproto.RegisterEmitterServer(grpcServer, srv)

	addr := listener.Addr().String()
	// log.Printf("Сервер слушает на %s...", addr)

	stop := func() {
		grpcServer.Stop()

		lisErr := listener.Close()
		if lisErr != nil && !strings.Contains(lisErr.Error(), "use of closed network connection") {
			t.Error(lisErr)
		}
	}

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Error().Err(err).Msg("Ошибка gRPC сервера")
			t.Error(err)
		}
	}()

	time.Sleep(time.Millisecond * 100) // Даем серверу время запуститься

	return addr, stop
}

func TestEmitterCall_PropertyBased(t *testing.T) {
	rapid.Check(t, func(tr *rapid.T) {
		t.Run(tr.Name(), func(t *testing.T) {
			dbPath := t.TempDir()

			db, err := mdbx.NewMDBX(mdbxlog.New()).
				Path(dbPath).
				WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
					return kv.TableCfg{
						CheckpointBucket: {},
						ExternalTxBucket: {},
						BlocksBucket:     {},
					}
				}).
				Open()
			require.NoError(tr, err)

			tx, err := db.BeginRw(t.Context())
			if err != nil {
				t.Fatalf("DB: %v", err)
			}
			defer tx.Rollback()

			// Создаем сервер с MDBX
			srv := NewServer[*CustomTransaction[Receipt]](db, 1, nil)
			defer srv.appchainDB.Close()

			// Запускаем gRPC сервер и получаем динамический адрес
			addr, stopServer := startGRPCServer(tr, srv)
			defer stopServer() // Гарантируем закрытие сервера после теста

			// Генерируем случайное количество чекпоинтов (до 100)
			numCheckpoints := rapid.IntRange(1, 100).Draw(tr, "numCheckpoints")
			checkpointMap := make(map[uint64]apptypes.Checkpoint)

			// Заполняем уникальные LatestBlockNumber
			for len(checkpointMap) < numCheckpoints {
				blockNumber := rapid.Uint64().Draw(tr, "LatestBlockNumber")
				checkpointMap[blockNumber] = randomCheckpoint(tr, 1, blockNumber)
			}

			// Преобразуем map в список и записываем в MDBX
			checkpoints := make([]apptypes.Checkpoint, 0, len(checkpointMap))
			for _, chk := range checkpointMap {
				checkpoints = append(checkpoints, chk)
				if err = WriteCheckpoint(t.Context(), tx, chk); err != nil {
					tr.Fatalf("Ошибка записи чекпоинта: %v", err)
				}
			}

			err = tx.Commit()
			require.NoError(tr, err)

			// Сортируем чекпоинты по LatestBlockNumber
			sort.Slice(checkpoints, func(i, j int) bool {
				return checkpoints[i].BlockNumber < checkpoints[j].BlockNumber
			})

			// Подключение к серверу
			conn, err := grpc.NewClient(
				addr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			) // Используем динамический адрес
			if err != nil {
				t.Fatalf("Не удалось подключиться: %v", err)
			}

			defer func() {
				connErr := conn.Close()
				require.NoError(t, connErr)
			}()

			client := emitterproto.NewEmitterClient(conn)

			// Генерируем случайный запрос
			startBlock := rapid.Uint64().Draw(tr, "startBlock")
			limit := rapid.Uint32Range(1, uint32(len(checkpoints))).Draw(tr, "limit")

			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
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

			chainIDRes, err := client.GetChainID(ctx, &emptypb.Empty{})
			if err != nil {
				t.Fatalf("GetChainID: %v", err)
			}

			// ✅ Проверяем, что чекпоинты в ответе соответствуют условиям запроса
			expectedCheckpoints := make([]apptypes.Checkpoint, 0)

			for _, chk := range checkpoints {
				if chk.BlockNumber >= startBlock {
					expectedCheckpoints = append(expectedCheckpoints, chk)
					if len(expectedCheckpoints) >= int(limit) {
						break
					}
				}
			}

			// ✅ Проверяем, что количество чекпоинтов соответствует лимиту
			if len(res.GetCheckpoints()) != len(expectedCheckpoints) {
				t.Fatalf(
					"Ошибка: ожидалось %d чекпоинтов, получено %d",
					len(expectedCheckpoints),
					len(res.GetCheckpoints()),
				)
			}

			// ✅ Проверяем, что чекпоинты отсортированы по LatestBlockNumber
			prevBlockNumber := uint64(0)

			for i, chk := range res.GetCheckpoints() {
				if chk.GetLatestBlockNumber() < startBlock {
					t.Fatalf(
						"Ошибка: чекпоинт %d меньше стартового блока %d",
						chk.GetLatestBlockNumber(),
						startBlock,
					)
				}

				if chk.GetLatestBlockNumber() < prevBlockNumber {
					t.Fatal("Ошибка: чекпоинты не отсортированы по LatestBlockNumber")
				}

				prevBlockNumber = chk.GetLatestBlockNumber()

				// ✅ Проверяем, что данные чекпоинта совпадают с ожидаемыми
				expected := expectedCheckpoints[i]
				if chainIDRes.GetChainId() != expected.ChainID ||
					!bytes.Equal(chk.GetStateRoot(), expected.StateRoot[:]) ||
					!bytes.Equal(chk.GetBlockHash(), expected.BlockHash[:]) ||
					!bytes.Equal(
						chk.GetExternalTxRootHash(),
						expected.ExternalTransactionsRoot[:],
					) {
					t.Fatal("Ошибка: данные чекпоинта не совпадают с записанными")
				}
			}
		})
	})
}

func TestGetExternalTransactions_PropertyBased(t *testing.T) {
	t.Skip()

	rapid.Check(t, func(tr *rapid.T) {
		t.Run(tr.Name(), func(t *testing.T) {
			dbPath := t.TempDir()

			db, err := mdbx.NewMDBX(mdbxlog.New()).
				Path(dbPath).
				WithTableCfg(func(_ kv.TableCfg) kv.TableCfg {
					return kv.TableCfg{
						CheckpointBucket: {},
						ExternalTxBucket: {},
						BlocksBucket:     {},
					}
				}).
				Open()

			require.NoError(tr, err)

			srv := NewServer[*CustomTransaction[Receipt]](db, 1, nil)
			defer srv.appchainDB.Close()

			tx, err := db.BeginRw(t.Context())
			if err != nil {
				tr.Fatalf("DB: %v", err)
			}

			addr, stopServer := startGRPCServer(tr, srv)
			defer stopServer()

			numBlocks := rapid.IntRange(1, 50).Draw(tr, "numBlocks")
			transactionMap := make(map[uint64][]apptypes.ExternalTransaction)

			for range numBlocks {
				blockNumber := rapid.Uint64().Draw(tr, "BlockNumber")
				numTx := rapid.IntRange(1, 20).Draw(tr, "numTxInBlock")

				for range numTx {
					tx := apptypes.ExternalTransaction{
						ChainID: rapid.Uint64().Draw(tr, "ChainId"),
						Tx:      rapid.SliceOfN(rapid.Byte(), 64, 64).Draw(tr, "Tx"),
					}
					transactionMap[blockNumber] = append(transactionMap[blockNumber], tx)
				}

				if _, err = WriteExternalTransactions(tx, blockNumber, transactionMap[blockNumber]); err != nil {
					tr.Fatalf("Ошибка записи транзакции: %v", err)
				}
			}

			err = tx.Commit()
			require.NoError(tr, err)

			conn, err := grpc.NewClient(
				addr,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				tr.Fatalf("Не удалось подключиться: %v", err)
			}

			defer func() {
				connErr := conn.Close()
				require.NoError(tr, connErr)
			}()

			client := emitterproto.NewEmitterClient(conn)

			startBlock := rapid.Uint64().Draw(tr, "startBlock")
			limit := rapid.Uint32Range(1, 100).Draw(tr, "limit")

			req := &emitterproto.GetExternalTransactionsRequest{
				LatestPreviousBlockNumber: startBlock,
				Limit:                     &limit,
			}

			res, err := client.GetExternalTransactions(t.Context(), req)
			if err != nil {
				tr.Fatalf("Ошибка вызова GetExternalTransactions: %v", err)
			}

			for _, blk := range res.GetBlocks() {
				if blk.GetBlockNumber() < startBlock {
					tr.Fatalf(
						"Ошибка: блок %d меньше startBlock %d",
						blk.GetBlockNumber(),
						startBlock,
					)
				}

				if txs, exists := transactionMap[blk.GetBlockNumber()]; exists {
					if len(blk.GetExternalTransactions()) != len(txs) {
						tr.Fatalf(
							"Ошибка: несоответствие количества транзакций для блока %d",
							blk.GetBlockNumber(),
						)
					}
				} else {
					tr.Fatalf("Ошибка: блок %d не найден в ожидаемых данных", blk.GetBlockNumber())
				}
			}
		})
	})
}

// CustomTransaction - тестовая структура транзакции
type CustomTransaction[R Receipt] struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Value int    `json:"value"`
}

func (c *CustomTransaction[R]) Unmarshal(b []byte) error {
	return json.Unmarshal(b, c)
}

func (c *CustomTransaction[R]) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

func (c *CustomTransaction[R]) Hash() [32]byte {
	s := c.From + c.To + strconv.Itoa(c.Value)

	return sha256.Sum256([]byte(s))
}

func (*CustomTransaction[R]) Process(
	_ kv.RwTx,
) (res R, txs []apptypes.ExternalTransaction, err error) {
	return
}

type Receipt struct{}

func (r Receipt) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func (r Receipt) Unmarshal(data []byte) error {
	return json.Unmarshal(data, &r)
}

func (Receipt) TxHash() [32]byte {
	return [32]byte{}
}

func (Receipt) Status() apptypes.TxReceiptStatus {
	return apptypes.ReceiptConfirmed
}
