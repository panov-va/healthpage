package api

import (
	"context"
	"errors"
	"net"
	"time"
)

// publicDNSServers — резолверы, к которым обращаемся напрямую при верификации CNAME кастомных
// доменов (этап 4.3), в обход /etc/resolv.conf контейнера. Обнаружено на проде (2026-07-22):
// резолвер хоста (Docker embedded DNS → апстрим VPS) может ещё не видеть свежесозданную запись
// клиента (NXDOMAIN/негативный кэш), хотя публичные DNS уже отдают верный ответ — иначе клиенту
// придётся объяснять, почему его собственный `dig` проходит, а наша верификация — нет.
var publicDNSServers = []string{"8.8.8.8:53", "1.1.1.1:53"}

type cnameLookupFunc func(ctx context.Context, host string) (string, error)

// newPublicDNSResolver возвращает CNAME-резолвер поверх publicDNSServers.
func newPublicDNSResolver() cnameLookupFunc {
	lookups := make([]cnameLookupFunc, len(publicDNSServers))
	for i, addr := range publicDNSServers {
		addr := addr
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{Timeout: 5 * time.Second}
				return d.DialContext(ctx, network, addr)
			},
		}
		lookups[i] = r.LookupCNAME
	}
	return chainResolvers(lookups)
}

// chainResolvers опрашивает lookups по очереди и возвращает первый успешный ответ. Определённый
// отрицательный ответ (домен/CNAME не найден) возвращается сразу — не гадаем дальше по другим
// резолверам, чтобы не подхватить case, где один резолвер отстаёт и держит устаревший CNAME.
// К следующему в списке переходим только при сетевой ошибке/таймауте текущего.
func chainResolvers(lookups []cnameLookupFunc) cnameLookupFunc {
	return func(ctx context.Context, host string) (string, error) {
		var lastErr error
		for _, lookup := range lookups {
			cname, err := lookup(ctx, host)
			if err == nil {
				return cname, nil
			}
			lastErr = err
			var dnsErr *net.DNSError
			if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
				return "", err
			}
		}
		return "", lastErr
	}
}
