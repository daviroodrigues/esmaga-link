package idgen

import (
	"context"
	"errors"
	"fmt"
	"math/bits"
	"math/rand/v2"
	"sync"

	"github.com/redis/go-redis/v9"
)

const (
	DefaultUniverseSize62_5 uint64 = 916132832
	DefaultHullDobellA5     uint64 = 566165401
	DefaultHullDobellC5     uint64 = 271828183

	DefaultUniverseSize62_6 uint64 = 56800235584

	DefaultHullDobellA uint64 = 3141592701

	DefaultHullDobellC uint64 = 2718281829

	DefaultUniverseSize62_7 uint64 = 3521614606208
	DefaultHullDobellA7     uint64 = 2176510000001
	DefaultHullDobellC7     uint64 = 1414213562373
)

type EraConfig struct {
	CodeLength int
	EraStart   uint64
	EraSize    uint64
	Multiplier uint64
	Increment  uint64
	Seed       uint64
}

func newHullDobellEra(codeLength int, size, a, c uint64) (EraConfig, error) {
	era := EraConfig{
		CodeLength: codeLength,
		EraStart:   0,
		EraSize:    size,
		Multiplier: a,
		Increment:  c,
		Seed:       0,
	}
	if !IsHullDobellValid(era.Multiplier, era.Increment, era.EraSize) {
		return EraConfig{}, errors.New("constantes LCG da era não satisfazem o Teorema de Hull-Dobell")
	}
	return era, nil
}

func DefaultEra5Chars() (EraConfig, error) {
	return newHullDobellEra(5, DefaultUniverseSize62_5, DefaultHullDobellA5, DefaultHullDobellC5)
}

func DefaultEra6Chars() (EraConfig, error) {
	return newHullDobellEra(6, DefaultUniverseSize62_6, DefaultHullDobellA, DefaultHullDobellC)
}

func DefaultEra7Chars() (EraConfig, error) {
	return newHullDobellEra(7, DefaultUniverseSize62_7, DefaultHullDobellA7, DefaultHullDobellC7)
}

func DefaultEras() ([]EraConfig, error) {
	e5, err := DefaultEra5Chars()
	if err != nil {
		return nil, err
	}
	e6, err := DefaultEra6Chars()
	if err != nil {
		return nil, err
	}
	e7, err := DefaultEra7Chars()
	if err != nil {
		return nil, err
	}
	return []EraConfig{e5, e6, e7}, nil
}

func (e EraConfig) Obfuscate(seqID uint64) uint64 {
	if e.EraSize == 0 {
		return seqID + e.EraStart
	}
	rem := seqID % e.EraSize
	if e.Increment > 0 {
		val := lcgNth(e.Seed, e.Multiplier, e.Increment, e.EraSize, rem)
		return val + e.EraStart
	}
	val := mulMod(rem, e.Multiplier, e.EraSize)
	return val + e.EraStart
}

func lcgNth(seed, a, c, m, n uint64) uint64 {
	if m == 0 {
		return 0
	}
	a %= m
	c %= m
	seed %= m

	resK := uint64(1)
	resB := uint64(0)
	curK := a
	curB := c

	for n > 0 {
		if n&1 == 1 {
			resK = mulMod(curK, resK, m)
			resB = (mulMod(curK, resB, m) + curB) % m
		}
		curB = (mulMod(curK, curB, m) + curB) % m
		curK = mulMod(curK, curK, m)
		n >>= 1
	}

	return (mulMod(resK, seed, m) + resB) % m
}

func mulMod(a, b, m uint64) uint64 {
	if m == 0 {
		return 0
	}
	a %= m
	b %= m
	hi, lo := bits.Mul64(a, b)
	_, rem := bits.Div64(hi, lo, m)
	return rem
}

type CheckpointStorage interface {
	GetCheckpoint(ctx context.Context, sequenceName string) (int64, error)
	SaveCheckpoint(ctx context.Context, sequenceName string, allocatedID int64) error
}

type IDGenerator struct {
	mu        sync.Mutex
	redis     *redis.Client
	redisKey  string
	batchSize uint64
	batch     []uint64
	batchIdx  int
	eras      []EraConfig
	encoder   *Base62Encoder
	storage   CheckpointStorage
}

func NewIDGenerator(
	rdb *redis.Client,
	redisKey string,
	batchSize uint64,
	era EraConfig,
	encoder *Base62Encoder,
	storage CheckpointStorage,
) (*IDGenerator, error) {
	return NewMultiEraIDGenerator(rdb, redisKey, batchSize, []EraConfig{era}, encoder, storage)
}

func NewMultiEraIDGenerator(
	rdb *redis.Client,
	redisKey string,
	batchSize uint64,
	eras []EraConfig,
	encoder *Base62Encoder,
	storage CheckpointStorage,
) (*IDGenerator, error) {
	if rdb == nil {
		return nil, errors.New("cliente redis não pode ser nulo")
	}
	if len(eras) == 0 {
		return nil, errors.New("ao menos uma era deve ser configurada")
	}
	if batchSize == 0 {
		batchSize = 1000
	}
	for _, era := range eras {
		if era.Increment > 0 {
			if !IsHullDobellValid(era.Multiplier, era.Increment, era.EraSize) {
				return nil, errors.New("as constantes da era não satisfazem o Teorema de Hull-Dobell")
			}
		} else if !IsCoprime(era.Multiplier, era.EraSize) {
			return nil, errors.New("o multiplicador e o tamanho da era devem ser coprimos")
		}
	}

	erasCopy := make([]EraConfig, len(eras))
	copy(erasCopy, eras)

	return &IDGenerator{
		redis:     rdb,
		redisKey:  redisKey,
		batchSize: batchSize,
		batch:     make([]uint64, 0, batchSize),
		eras:      erasCopy,
		encoder:   encoder,
		storage:   storage,
	}, nil
}

func (g *IDGenerator) prepareBatch(startID uint64) {
	g.batch = g.batch[:cap(g.batch)]
	if uint64(len(g.batch)) != g.batchSize {
		g.batch = make([]uint64, g.batchSize)
	}
	for i := uint64(0); i < g.batchSize; i++ {
		g.batch[i] = startID + i
	}
	rand.Shuffle(len(g.batch), func(i, j int) {
		g.batch[i], g.batch[j] = g.batch[j], g.batch[i]
	})
	g.batchIdx = 0
}

func (g *IDGenerator) InitOrRecover(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.storage == nil {
		return nil
	}

	exists, err := g.redis.Exists(ctx, g.redisKey).Result()
	if err != nil {
		return fmt.Errorf("falha ao verificar existência da chave de id no redis: %w", err)
	}

	checkpoint, err := g.storage.GetCheckpoint(ctx, g.redisKey)
	if err != nil {
		return fmt.Errorf("falha ao ler checkpoint da storage durável: %w", err)
	}

	if exists == 0 {
		safeStart := checkpoint + int64(g.batchSize*2)
		set, err := g.redis.SetNX(ctx, g.redisKey, safeStart, 0).Result()
		if err != nil {
			return fmt.Errorf("falha ao inicializar chave no redis: %w", err)
		}
		if set {
			if err := g.storage.SaveCheckpoint(ctx, g.redisKey, safeStart); err != nil {
				return fmt.Errorf("falha ao persistir checkpoint de recuperação no cassandra: %w", err)
			}
		}
	} else {

		val, err := g.redis.Get(ctx, g.redisKey).Int64()
		if err == nil && val > checkpoint {
			_ = g.storage.SaveCheckpoint(ctx, g.redisKey, val)
		}
	}

	return nil
}

func (g *IDGenerator) NextID(ctx context.Context) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.batchIdx >= len(g.batch) {
		newMax, err := g.redis.IncrBy(ctx, g.redisKey, int64(g.batchSize)).Result()
		if err != nil {
			return "", fmt.Errorf("falha ao reservar lote no redis: %w", err)
		}

		startID := uint64(newMax) - g.batchSize + 1
		g.prepareBatch(startID)

		if g.storage != nil {
			_ = g.storage.SaveCheckpoint(ctx, g.redisKey, newMax)
		}
	}

	seqID := g.batch[g.batchIdx]
	g.batchIdx++

	return g.encodeSeqID(seqID), nil
}

func (g *IDGenerator) encodeSeqID(seqID uint64) string {
	remaining := seqID
	if remaining > 0 {
		remaining--
	}

	for _, era := range g.eras {
		if remaining < era.EraSize || era.EraSize == 0 {
			obfuscatedID := era.Obfuscate(remaining)
			if era.CodeLength > 0 {
				return g.encoder.EncodeWithPad(obfuscatedID, era.CodeLength)
			}
			return g.encoder.Encode(obfuscatedID)
		}
		remaining -= era.EraSize
	}

	lastEra := g.eras[len(g.eras)-1]
	obfuscatedID := lastEra.Obfuscate(remaining)
	if lastEra.CodeLength > 0 {
		return g.encoder.EncodeWithPad(obfuscatedID, lastEra.CodeLength)
	}
	return g.encoder.Encode(obfuscatedID)
}
