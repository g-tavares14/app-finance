# CLAUDE.md

Convenções do projeto `app-finance` (API em Go com Postgres).

## Idioma

- **Código em inglês:** nomes de variáveis, funções, tipos, structs, pacotes, arquivos, colunas e tabelas.
- **Mensagens dentro do código em inglês:** mensagens de erro (`fmt.Errorf`), logs (`slog`) e respostas HTTP.
- **Comentários em português.**
- **Mensagens de commit em português.** O assunto vai no imperativo (ex.: `Adiciona pool de conexões Postgres`), e o corpo explica o porquê.

## Estrutura

Fluxo das camadas: `handlers` → `usecases` → `repositories` → banco.

- `cmd/api`: ponto de entrada da API (lê `DATABASE_URL` e sobe o servidor na porta 8080).
- `cmd/client`: script que cria um usuário de exemplo chamando `POST /users` da API.
- `internal/db`: abre o pool de conexões (`database/sql` com o driver `pgx`).
- `internal/repositories`: acesso a dados. Cada entidade fica em um subpacote (ex.: `users`).
- `internal/models`: structs de domínio e de requisição/resposta.

## Convenções de código

- Logs com `log/slog`, registrados uma única vez na borda (handlers e `main`). As camadas de baixo só embrulham o erro com `fmt.Errorf("...: %w", err)`.
- Queries SQL usam placeholders do Postgres (`$1`, `$2`, ...) e nunca concatenam valores.

## Comandos

- Banco local: `docker compose up -d` (Postgres 17 na porta 5432).
- Build e verificação: `go build ./... && go vet ./...`.
- DSN padrão: `postgres://finance:finance@localhost:5432/finance?sslmode=disable` (sobrescreva com `DATABASE_URL`).
