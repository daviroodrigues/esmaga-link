package idgen

import (
	"testing"
)

func TestCoprimesAndGoldenRatio(t *testing.T) {

	var eraSize uint64 = 55884102752
	coprime, err := FindGoldenCoprime(eraSize)
	if err != nil {
		t.Fatalf("Erro ao buscar coprimo: %v", err)
	}

	if !IsCoprime(coprime, eraSize) {
		t.Errorf("Coprimo calculado %d não é coprimo com eraSize %d", coprime, eraSize)
	}

	inv, err := ModularInverse(coprime, eraSize)
	if err != nil {
		t.Fatalf("Erro ao calcular inverso modular: %v", err)
	}

	check := mulMod(coprime, inv, eraSize)
	if check != 1 {
		t.Errorf("Inverso modular falhou: (coprime * inv) %% eraSize = %d, esperado 1", check)
	}
}

func TestShuffleAlphabetDeterministic(t *testing.T) {
	seed := "minha-seed-secreta-2026"
	alpha1 := ShuffleAlphabet(seed)
	alpha2 := ShuffleAlphabet(seed)

	if alpha1 != alpha2 {
		t.Fatalf("ShuffleAlphabet não é determinístico para a mesma seed: %s != %s", alpha1, alpha2)
	}

	if len(alpha1) != 62 {
		t.Fatalf("Tamanho do alfabeto embaralhado incorreto: %d", len(alpha1))
	}

	if alpha1 == DefaultAlphabet {
		t.Errorf("Alfabeto não foi embaralhado, permaneceu idêntico ao padrão")
	}

	encoder, err := NewBase62EncoderWithSeed(seed, 6)
	if err != nil {
		t.Fatalf("Erro ao criar encoder com seed: %v", err)
	}

	num := uint64(123456789)
	enc := encoder.Encode(num)
	dec, err := encoder.Decode(enc)
	if err != nil || dec != num {
		t.Fatalf("Decode(Encode(%d)) com seed falhou: %d != %d", num, dec, num)
	}
}

func TestEraObfuscationNoCollisionsSmallSet(t *testing.T) {

	var eraSize uint64 = 10007
	coprime, err := FindGoldenCoprime(eraSize)
	if err != nil {
		t.Fatalf("Erro ao buscar coprimo: %v", err)
	}

	era := EraConfig{
		EraStart:   0,
		EraSize:    eraSize,
		Multiplier: coprime,
	}

	seen := make(map[uint64]bool, eraSize)
	for i := uint64(0); i < eraSize; i++ {
		obf := era.Obfuscate(i)
		if seen[obf] {
			t.Fatalf("Colisão detectada para seqID %d -> obf %d", i, obf)
		}
		seen[obf] = true
	}
}

func TestHullDobellValidation(t *testing.T) {

	if IsHullDobellValid(3141592653, DefaultHullDobellC, DefaultUniverseSize62_6) {
		t.Fatalf("Esperava falha para A=3141592653 no universo 62^6")
	}

	if !IsHullDobellValid(DefaultHullDobellA, DefaultHullDobellC, DefaultUniverseSize62_6) {
		t.Fatalf("Constantes padrão de Hull-Dobell deveriam ser válidas para 62^6")
	}

	era, err := DefaultEra6Chars()
	if err != nil {
		t.Fatalf("DefaultEra6Chars() retornou erro inesperado: %v", err)
	}
	if era.EraSize != DefaultUniverseSize62_6 || era.Multiplier != DefaultHullDobellA || era.Increment != DefaultHullDobellC {
		t.Fatalf("DefaultEra6Chars() retornou configuração inesperada: %+v", era)
	}
}

func TestHullDobellFullPeriodSmallUniverse(t *testing.T) {

	var eraSize uint64 = 62 * 62
	var a uint64 = 124*17 + 1
	var c uint64 = 1073

	if !IsHullDobellValid(a, c, eraSize) {
		t.Fatalf("Constantes de teste inválidas para Hull-Dobell")
	}

	era := EraConfig{
		EraStart:   0,
		EraSize:    eraSize,
		Multiplier: a,
		Increment:  c,
		Seed:       0,
	}

	seen := make(map[uint64]bool, eraSize)
	for i := uint64(0); i < eraSize; i++ {
		obf := era.Obfuscate(i)
		if obf >= eraSize {
			t.Fatalf("Valor ofuscado fora do universo: %d >= %d", obf, eraSize)
		}
		if seen[obf] {
			t.Fatalf("Colisão detectada para seqID %d -> obf %d", i, obf)
		}
		seen[obf] = true
	}
	if uint64(len(seen)) != eraSize {
		t.Fatalf("Período incompleto: %d != %d", len(seen), eraSize)
	}
}

func TestLCGNthMatchesIterative(t *testing.T) {
	era, err := DefaultEra6Chars()
	if err != nil {
		t.Fatalf("Erro ao criar Era padrão: %v", err)
	}

	iterative := era.Seed % era.EraSize
	for n := uint64(0); n < 5000; n++ {
		got := era.Obfuscate(n)
		if got != iterative {
			t.Fatalf("Divergência em n=%d: lcgNth=%d, iterativo=%d", n, got, iterative)
		}
		iterative = (mulMod(era.Multiplier, iterative, era.EraSize) + era.Increment) % era.EraSize
	}
}

func TestPrepareBatchShufflesWithoutCollisions(t *testing.T) {
	eras, err := DefaultEras()
	if err != nil {
		t.Fatalf("Erro ao criar Eras padrão: %v", err)
	}
	enc, err := NewBase62EncoderWithSeed("test-seed-batch", 5)
	if err != nil {
		t.Fatalf("Erro ao criar Base62Encoder: %v", err)
	}

	const batchSize uint64 = 1000
	gen := &IDGenerator{
		batchSize: batchSize,
		batch:     make([]uint64, 0, batchSize),
		eras:      eras,
		encoder:   enc,
	}

	gen.prepareBatch(1)

	if uint64(len(gen.batch)) != batchSize {
		t.Fatalf("Tamanho do lote incorreto: %d != %d", len(gen.batch), batchSize)
	}

	seenIDs := make(map[uint64]bool, batchSize)
	seenCodes := make(map[string]bool, batchSize)
	sequentialCount := 0

	for i, seqID := range gen.batch {
		if seqID < 1 || seqID > batchSize {
			t.Fatalf("ID fora do intervalo do lote: %d", seqID)
		}
		if seenIDs[seqID] {
			t.Fatalf("ID duplicado dentro do lote: %d", seqID)
		}
		seenIDs[seqID] = true

		if seqID == uint64(i+1) {
			sequentialCount++
		}

		code := gen.encodeSeqID(seqID)
		if len(code) != 5 {
			t.Fatalf("Tamanho de short_code inválido na Era 5: %s (len=%d)", code, len(code))
		}
		if seenCodes[code] {
			t.Fatalf("Colisão de short_code no lote: %s", code)
		}
		seenCodes[code] = true
	}

	if sequentialCount == int(batchSize) {
		t.Fatalf("O lote não foi aleatorizado (todos os %d IDs permaneceram em ordem sequencial)", batchSize)
	}
}

func TestMultiEraProgression5To6To7(t *testing.T) {
	eras, err := DefaultEras()
	if err != nil {
		t.Fatalf("DefaultEras() falhou: %v", err)
	}
	enc, err := NewBase62EncoderWithSeed("test-seed-eras", 5)
	if err != nil {
		t.Fatalf("NewBase62EncoderWithSeed() falhou: %v", err)
	}

	gen := &IDGenerator{
		eras:    eras,
		encoder: enc,
	}

	if got := gen.encodeSeqID(1); len(got) != 5 {
		t.Fatalf("seqID=1 deveria gerar código de 5 chars, obteve %q (len=%d)", got, len(got))
	}
	if got := gen.encodeSeqID(DefaultUniverseSize62_5); len(got) != 5 {
		t.Fatalf("último ID da Era 5 deveria gerar 5 chars, obteve %q (len=%d)", got, len(got))
	}

	if got := gen.encodeSeqID(DefaultUniverseSize62_5 + 1); len(got) != 6 {
		t.Fatalf("primeiro ID da Era 6 deveria gerar 6 chars, obteve %q (len=%d)", got, len(got))
	}
	if got := gen.encodeSeqID(DefaultUniverseSize62_5 + DefaultUniverseSize62_6); len(got) != 6 {
		t.Fatalf("último ID da Era 6 deveria gerar 6 chars, obteve %q (len=%d)", got, len(got))
	}

	if got := gen.encodeSeqID(DefaultUniverseSize62_5 + DefaultUniverseSize62_6 + 1); len(got) != 7 {
		t.Fatalf("primeiro ID da Era 7 deveria gerar 7 chars, obteve %q (len=%d)", got, len(got))
	}
}
