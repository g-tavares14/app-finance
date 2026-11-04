# go-auth-api

API de autenticação em Go com PostgreSQL: cadastro de usuários, login com JWT e gestão da própria conta (ver, editar e excluir), com proteção contra força bruta.

Feita só com a biblioteca padrão para HTTP (`net/http`, roteamento do Go 1.22+), `database/sql` com o driver `pgx`, `bcrypt` para senhas e `golang-jwt` para tokens.

## Endpoints

| Método | Rota | Autenticação | Corpo | Resposta |
|---|---|---|---|---|
| `GET` | `/health` | — | — | `200` `{"status":"ok"}` |
| `POST` | `/users` | — | `first_name`, `last_name`, `email`, `password` | `201` `{"new_user_id":"<uuid>"}` |
| `POST` | `/login` | — | `email`, `password` | `200` `{"token":"<jwt>"}` |
| `GET` | `/users/me` | Bearer | — | `200` `{"id","first_name","last_name","email"}` |
| `PATCH` | `/users/me` | Bearer | qualquer um de `first_name`, `last_name`, `email`, `password` (+ `current_password` ao trocar e-mail ou senha) | `204` |
| `DELETE` | `/users/me` | Bearer | `password` | `204` |

Rotas autenticadas exigem o cabeçalho `Authorization: Bearer <token>`. Os erros voltam como texto simples, com o status HTTP correspondente:

| Status | Quando |
|---|---|
| `400` | JSON inválido ou dados que não passam na validação |
| `401` | credenciais erradas, token ausente, inválido ou expirado |
| `404` | usuário não encontrado |
| `409` | e-mail já cadastrado |
| `413` | corpo da requisição maior que 1 MiB |
| `429` | tentativas demais; o cabeçalho `Retry-After` diz quantos segundos esperar |

## Segurança

**Senhas**
- Guardadas com `bcrypt`. A senha precisa ter de 5 a 72 bytes (72 é o limite do bcrypt).
- No login com um e-mail que não existe, a API compara a senha com um hash falso mesmo assim. Assim o tempo de resposta é o mesmo e não revela quais e-mails estão cadastrados.
- Trocar e-mail ou senha, e excluir a conta, exige a senha atual.

**Tokens**
- JWT assinado com HS256, válido por 24 horas. Só HS256 é aceito na validação.
- O token carrega a claim `pwv`, um HMAC do hash da senha. Quando a senha muda, o HMAC muda junto, e todos os tokens emitidos antes deixam de valer.
- A cada requisição autenticada o usuário é buscado de novo no banco, então um token de uma conta excluída é recusado.

**Força bruta (rate limit)**
- Só tentativas que falham por credencial errada contam. Uma tentativa bem-sucedida libera a vaga.
- No login, três limites valem ao mesmo tempo, cada um numa janela de 15 minutos:

  | Chave | Falhas permitidas |
  |---|---|
  | e-mail | 25 |
  | e-mail + IP | 5 |
  | IP | 5 |

- Excluir a conta e trocar e-mail ou senha têm limite de 5 falhas por usuário a cada 15 minutos.
- IPs privados e de loopback não entram nas chaves de IP. Nesse caso vale um único limite, de 5 falhas por e-mail. IPv6 é agrupado por prefixo `/64`.
- Atrás de um proxy reverso, `TRUST_PROXY=true` faz a API usar o último endereço de `X-Forwarded-For` como IP do cliente.

**Dados**
- E-mails são normalizados para minúsculas. Uma constraint `CHECK` no banco garante isso.
- Todas as queries usam placeholders (`$1`, `$2`, ...), nunca concatenação.
- O servidor tem timeouts de leitura, escrita e ociosidade, e desliga de forma graciosa ao receber `SIGINT` ou `SIGTERM`.

## Como rodar

Pré-requisitos: Docker. Para rodar a API fora do Docker, Go 1.27 ou mais recente.

### Tudo no Docker

```bash
echo "JWT_SECRET=$(openssl rand -hex 32)" > .env
docker compose up -d --build
curl localhost:8080/health
```

O compose sobe o Postgres 17 e a API na porta `8080`. A API espera o banco ficar saudável e aplica as migrations ao iniciar.

### API local, banco no Docker

```bash
docker compose up -d db
JWT_SECRET=$(openssl rand -hex 32) go run ./cmd/api
```

### Exemplo de uso

```bash
# cadastro
curl -X POST localhost:8080/users \
  -d '{"first_name":"Ana","last_name":"Silva","email":"ana@example.com","password":"senha-forte"}'

# login
TOKEN=$(curl -s -X POST localhost:8080/login \
  -d '{"email":"ana@example.com","password":"senha-forte"}' | jq -r .token)

# dados do usuário logado
curl localhost:8080/users/me -H "Authorization: Bearer $TOKEN"

# troca de senha (invalida o token atual)
curl -X PATCH localhost:8080/users/me -H "Authorization: Bearer $TOKEN" \
  -d '{"password":"outra-senha","current_password":"senha-forte"}'
```

`go run ./cmd/client` também cria um usuário de exemplo chamando `POST /users`.

## Configuração

| Variável | Obrigatória | Padrão | Descrição |
|---|---|---|---|
| `JWT_SECRET` | sim | — | Chave de assinatura dos tokens, com no mínimo 32 bytes |
| `DATABASE_URL` | não | `postgres://finance:finance@localhost:5432/finance?sslmode=disable` | DSN do Postgres |
| `TRUST_PROXY` | não | `false` | Usa `X-Forwarded-For` para descobrir o IP do cliente |

A porta é fixa em `8080`.

## Estrutura

```
cmd/
  api/            ponto de entrada: lê a configuração, conecta, migra e sobe o servidor
  client/         script que cria um usuário de exemplo
internal/
  handlers/       rotas HTTP, rate limit e mapeamento de erros para status
  middleware/     autenticação (JWT), log de requisições e limite de corpo
  usecases/       regras de negócio: validação, bcrypt e JWT
  repositories/   acesso ao Postgres (um subpacote por entidade)
  ratelimit/      limitador de tentativas em memória
  db/             pool de conexões e migrations embutidas
  models/         structs de domínio e de requisição/resposta
```

As camadas seguem o fluxo `handlers` → `usecases` → `repositories` → banco. O middleware de autenticação depende de uma interface pequena (`Authenticator`), não do pacote `usecases` diretamente.

As migrations ficam em `internal/db/migrations`, são embutidas no binário com `go:embed` e aplicadas com `golang-migrate` quando a API sobe.

A imagem Docker é multi-stage: compila um binário estático e o copia para uma imagem `distroless` que roda como usuário sem privilégios.

## Limitações conhecidas

- O rate limit fica em memória: zera quando a API reinicia e não é compartilhado entre várias instâncias.
- Não há refresh token nem revogação de um token específico. Um token só deixa de valer quando expira, quando a senha muda ou quando a conta é excluída.
- Ainda não há testes automatizados nem CI.
