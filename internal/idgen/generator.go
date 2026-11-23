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
	// DefaultUniverseSize62_6 representa o universo de 62^6 códigos (2^6 * 31^6)
	DefaultUniverseSize62_6 uint64 = 56800235584
	// DefaultHullDobellA é o multiplicador ajustado para satisfazer (A - 1) % 124 == 0
	DefaultHullDobellA uint64 = 3141592701
	// DefaultHullDobellC é o incremento coprimo com 62^6
	DefaultHullDobellC uint64 = 2718281829
)

// EraConfig define os parâmetros matemáticos de uma Era
type EraConfig struct {
	EraStart   uint64
	EraSize    uint64
	Multiplier uint64 // Número coprimo com EraSize ou multiplicador A do LCG
	Increment  uint64 // Incremento C do LCG (quando > 0, ativa salto caótico Hull-Dobell)
	Seed       uint64 // Semente inicial do LCG (X_0)
}

// DefaultEra6Chars cria uma configuração padrão de Era para códigos de 6 caracteres (~56.8 bilhões de links)
// utilizando LCG com constantes validadas pelo Teorema de Hull-Dobell (período completo sem colisões).
func DefaultEra6Chars() (EraConfig, error) {
	era := EraConfig{
		EraStart:   0,
		EraSize:    DefaultUniverseSize62_6,
		Multiplier: DefaultHullDobellA,
		Increment:  DefaultHullDobellC,
		Seed:       0,
	}
	if !IsHullDobellValid(era.Multiplier, era.Increment, era.EraSize) {
		return EraConfig{}, errors.New("constantes LCG da era não satisfazem o Teorema de Hull-Dobell")
	}

	return era, nil
}

// Obfuscate aplica a transformação bijetora (LCG caótico Hull-Dobell se Increment > 0, ou Fibonacci Hashing).
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

// lcgNth calcula o n-ésimo estado do LCG T^n(seed) onde T(x) = (a*x + c) mod m em tempo O(log n).
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

// mulMod calcula (a * b) % m evitando overflow em uint64 usando aritmética de 128 bits.
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

// CheckpointStorage define a interface para persistência do teto de IDs em armazenamento durável (SSOT)
type CheckpointStorage interface {
	GetCheckpoint(ctx context.Context, sequenceName string) (int64, error)
	SaveCheckpoint(ctx context.Context, sequenceName string, allocatedID int64) error
}

// IDGenerator coordena a reserva de IDs em lote no Redis, aleatorização intralote e a geração do short_code
type IDGenerator struct {
	mu        sync.Mutex
	redis     *redis.Client
	redisKey  string
	batchSize uint64
	batch     []uint64
	batchIdx  int
	era       EraConfig
	encoder   *Base62Encoder
	storage   CheckpointStorage
}

// NewIDGenerator instancia o gerador com alocação em bloco e aleatorização intralote
func NewIDGenerator(
	rdb *redis.Client,
	redisKey string,
	batchSize uint64,
	era EraConfig,
	encoder *Base62Encoder,
	storage CheckpointStorage,
) (*IDGenerator, error) {
	if rdb == nil {
		return nil, errors.New("cliente redis não pode ser nulo")
	}
	if batchSize == 0 {
		batchSize = 1000
	}
	if era.Increment > 0 {
		if !IsHullDobellValid(era.Multiplier, era.Increment, era.EraSize) {
			return nil, errors.New("as constantes da era não satisfazem o Teorema de Hull-Dobell")
		}
	} else if !IsCoprime(era.Multiplier, era.EraSize) {
		return nil, errors.New("o multiplicador e o tamanho da era devem ser coprimos")
	}

	return &IDGenerator{
		redis:     rdb,
		redisKey:  redisKey,
		batchSize: batchSize,
		batch:     make([]uint64, 0, batchSize),
		era:       era,
		encoder:   encoder,
		storage:   storage,
	}, nil
}

// prepareBatch preenche o lote com os IDs sequenciais [startID, startID+batchSize-1]
// e aplica Fisher-Yates shuffle para aleatorizar a ordem dentro do lote antes da ofuscação.
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

// InitOrRecover sincroniza o Redis com a Storage durável (Cassandra) em caso de reinicialização ou desastre
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
		// Pior caso: Redis subiu vazio (perda de volume / primeiro boot).
		// Aplica margem de segurança (+ 2 lotes) para garantir que nenhum ID em trânsito colida
		safeStart := checkpoint + int64(g.batchSize*2)
		if err := g.redis.Set(ctx, g.redisKey, safeStart, 0).Err(); err != nil {
			return fmt.Errorf("falha ao inicializar chave no redis: %w", err)
		}
		if err := g.storage.SaveCheckpoint(ctx, g.redisKey, safeStart); err != nil {
			return fmt.Errorf("falha ao persistir checkpoint de recuperação no cassandra: %w", err)
		}
	} else {
		// Redis já possui a chave; se o valor for maior que o checkpoint, atualiza o Cassandra
		val, err := g.redis.Get(ctx, g.redisKey).Int64()
		if err == nil && val > checkpoint {
			_ = g.storage.SaveCheckpoint(ctx, g.redisKey, val)
		}
	}

	return nil
}

// NextID obtém o próximo código Base62 único e ofuscado de forma thread-safe
func (g *IDGenerator) NextID(ctx context.Context) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Se o lote local esgotou, reserva um novo lote no Redis e aleatoriza os IDs internamente
	if g.batchIdx >= len(g.batch) {
		newMax, err := g.redis.IncrBy(ctx, g.redisKey, int64(g.batchSize)).Result()
		if err != nil {
			return "", fmt.Errorf("falha ao reservar lote no redis: %w", err)
		}

		startID := uint64(newMax) - g.batchSize + 1
		g.prepareBatch(startID)

		// Checkpoint durável no Cassandra (SSOT)
		if g.storage != nil {
			_ = g.storage.SaveCheckpoint(ctx, g.redisKey, newMax)
		}
	}

	seqID := g.batch[g.batchIdx]
	g.batchIdx++

	// Ofusca o ID com a transformação caótica (Hull-Dobell LCG)
	obfuscatedID := g.era.Obfuscate(seqID)

	// Codifica em Base62
	return g.encoder.Encode(obfuscatedID), nil
}
