package security

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

var (
	ErrDisallowedHost = errors.New("o host de destino não é permitido (endereços locais ou privados são proibidos)")
	ErrSuspiciousURL  = errors.New("a URL foi sinalizada como potencialmente perigosa")
)

type IPResolver func(host string) ([]net.IP, error)

type URLValidator struct {
	blockedDomains []string
	resolver       IPResolver
}

func NewURLValidator(blockedDomains []string) *URLValidator {
	return &URLValidator{
		blockedDomains: blockedDomains,
		resolver:       net.LookupIP,
	}
}

func NewURLValidatorWithResolver(blockedDomains []string, resolver IPResolver) *URLValidator {
	return &URLValidator{
		blockedDomains: blockedDomains,
		resolver:       resolver,
	}
}

func (v *URLValidator) ValidateDestination(rawURL string) error {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return errors.New("formato de URL inválido")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("apenas protocolos HTTP e HTTPS são aceitos")
	}

	hostname := strings.ToLower(u.Hostname())
	if hostname == "" {
		return errors.New("hostname vazio")
	}

	if hostname == "localhost" || hostname == "metadata.google.internal" {
		return ErrDisallowedHost
	}

	for _, domain := range v.blockedDomains {
		if hostname == domain || strings.HasSuffix(hostname, "."+domain) {
			return ErrSuspiciousURL
		}
	}

	resolver := v.resolver
	if resolver == nil {
		resolver = net.LookupIP
	}

	if parsedIP := net.ParseIP(hostname); parsedIP != nil {
		if isPrivateOrLoopbackIP(parsedIP) {
			return ErrDisallowedHost
		}
		return nil
	}

	ips, err := resolver(hostname)
	if err != nil {

		return errors.New("não foi possível resolver o domínio informado")
	}

	for _, ip := range ips {
		if isPrivateOrLoopbackIP(ip) {
			return ErrDisallowedHost
		}
	}

	return nil
}

func isPrivateOrLoopbackIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"100.64.0.0/10",
		"fc00::/7",
		"fe80::/10",
	}

	for _, cidr := range privateCIDRs {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return true
		}
	}

	return false
}
