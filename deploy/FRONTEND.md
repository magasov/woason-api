# Фронт на том же сервере

Сейчас `https://woason.ru` отдаёт заглушку из `deploy/frontend/`. API — `https://api.woason.ru`.

## Что положить

1. Соберите Next.js (или другой фронт) с:
   - `NEXT_PUBLIC_API_URL=https://api.woason.ru`
   - WebSocket: `wss://api.woason.ru/ws`
2. Результат `out/` (static export) скопируйте в `deploy/frontend/` на сервере **вместо** текущей заглушки.
3. Перезапуск прокси:

```bash
cd /opt/woason/api
docker compose -f docker-compose.prod.yml exec caddy caddy reload --config /etc/caddy/Caddyfile
```

Если нужен Node-сервер (не static export), добавьте сервис `web` в `docker-compose.prod.yml` и в `deploy/Caddyfile` замените `file_server` на:

```
woason.ru {
	reverse_proxy web:3000
}
```

`www.woason.ru` сейчас смотрит на другой IP у регистратора. Чтобы заработал www, поставьте A-запись `www` на `2.27.201.121` и добавьте `www.woason.ru` в блок `woason.ru` в Caddyfile.
