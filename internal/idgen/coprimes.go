package idgen

import (
	"errors"
	"math"
	"math/big"
)

const (
	// GoldenRatioProportion é a proporção áurea (phi - 1)
	GoldenRatioProportion = 0.618033988749895
)

// GCD calcula o Máximo Divisor Comum entre dois números de 64 bits usando Euclides.
func GCD(a, b uint64) uint64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// IsCoprime verifica se dois números são coprimos (mdc == 1).
func IsCoprime(a, b uint64) bool {
	return GCD(a, b) == 1
}

// IsHullDobellValid verifica as 3 condições do Teorema de Hull-Dobell para que
// o LCG X_{n+1} = (a*X_n + c) mod m tenha período completo igual a m:
// 1. mdc(c, m) == 1
// 2. (a - 1) é divisível por todos os fatores primos de m
// 3. Se 4 divide m, então 4 divide (a - 1)
func IsHullDobellValid(a, c, m uint64) bool {
	if m <= 1 || a == 0 || c == 0 {
		return false
	}
	if !IsCoprime(c, m) {
		return false
	}

	aMinus1 := a - 1
	if m%4 == 0 && aMinus1%4 != 0 {
		return false
	}

	temp := m
	for p := uint64(2); p*p <= temp; p++ {
		if temp%p == 0 {
			if aMinus1%p != 0 {
				return false
			}
			for temp%p == 0 {
				temp /= p
			}
		}
	}
	if temp > 1 && aMinus1%temp != 0 {
		return false
	}

	return true
}

// FindGoldenCoprime encontra o número coprimo mais próximo da proporção áurea para o tamanho da era.
func FindGoldenCoprime(eraSize uint64) (uint64, error) {
	if eraSize <= 1 {
		return 0, errors.New("tamanho da era deve ser maior que 1")
	}

	target := uint64(math.Round(float64(eraSize) * GoldenRatioProportion))

	// Busca o coprimo mais próximo alternando para cima e para baixo
	delta := uint64(0)
	for {
		// Tenta para cima
		if target+delta < eraSize && IsCoprime(target+delta, eraSize) {
			return target + delta, nil
		}
		// Tenta para baixo
		if target >= delta && target-delta > 1 && IsCoprime(target-delta, eraSize) {
			return target - delta, nil
		}
		delta++
		if delta > eraSize/2 {
			break
		}
	}

	return 0, errors.New("não foi possível encontrar um coprimo adequado")
}

// ModularInverse calcula o inverso multiplicativo modular (a^-1 mod m)
func ModularInverse(a, m uint64) (uint64, error) {
	bigA := new(big.Int).SetUint64(a)
	bigM := new(big.Int).SetUint64(m)
	inv := new(big.Int).ModInverse(bigA, bigM)
	if inv == nil {
		return 0, errors.New("inverso modular não existe (não são coprimos)")
	}
	return inv.Uint64(), nil
}
