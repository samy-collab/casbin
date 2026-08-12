# API de tarefas com Gin e Casbin

API REST simples em Go para criar, listar, editar e excluir tarefas usando Gin e Casbin.

## Recursos

- CRUD de tarefas em memória.
- Autenticação simplificada por header `X-User`.
- ACL com permissões diretas por usuário.
- RBAC com funções `admin` e `user`.
- ABAC para permitir edição e exclusão por dono da tarefa.
- Middleware de autorização consultando o Casbin Enforcer em cada rota.
- Políticas persistidas em `authz/policy.csv`.
- Testes automatizados de acesso permitido e negado.

## Requisitos

- Go 1.26 ou superior.

## Instalação

```bash
go mod tidy
```

## Executar

```bash
go run .
```

A API ficará disponível em:

```text
http://localhost:8080
```

## Testar

```bash
go test ./...
```

## Usuários e permissões

As políticas ficam em `authz/policy.csv`:

- `admin`: pode criar, listar, ver, editar e excluir qualquer tarefa.
- `alice`: tem função `user` e também permissão ACL direta para criar tarefas.
- `bob`: tem função `user`, pode listar e ver tarefas, mas não pode criar.
- `owner`: regra ABAC especial que permite `PUT` e `DELETE` quando `X-User` é igual ao dono da tarefa.

## Endpoints

Todas as rotas exigem o header:

```http
X-User: alice
```

### Criar tarefa

```bash
curl -i -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -H 'X-User: alice' \
  -d '{"title":"Estudar Casbin","description":"Implementar ACL, RBAC e ABAC"}'
```

### Listar tarefas

```bash
curl -i http://localhost:8080/tasks \
  -H 'X-User: bob'
```

### Buscar tarefa

```bash
curl -i http://localhost:8080/tasks/1 \
  -H 'X-User: bob'
```

### Editar tarefa

Permitido para o dono da tarefa ou para `admin`.

```bash
curl -i -X PUT http://localhost:8080/tasks/1 \
  -H 'Content-Type: application/json' \
  -H 'X-User: alice' \
  -d '{"title":"Estudar Casbin","description":"Finalizar exemplos","done":true}'
```

### Excluir tarefa

Permitido para o dono da tarefa ou para `admin`.

```bash
curl -i -X DELETE http://localhost:8080/tasks/1 \
  -H 'X-User: alice'
```

## Exemplos de acesso negado

Bob não possui permissão ACL para criar tarefas:

```bash
curl -i -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -H 'X-User: bob' \
  -d '{"title":"Tarefa do Bob"}'
```

Bob também não pode editar tarefas de Alice:

```bash
curl -i -X PUT http://localhost:8080/tasks/1 \
  -H 'Content-Type: application/json' \
  -H 'X-User: bob' \
  -d '{"title":"Tentativa indevida"}'
```

## Modelo de autorização

O modelo fica em `authz/model.conf`.

Cada requisição enviada ao Casbin usa:

```text
sub = usuário do header X-User
obj = rota Gin, por exemplo /tasks ou /tasks/:id
act = método HTTP
owner = dono da tarefa, quando a rota tem :id
```

O matcher permite acesso quando existe uma política compatível por usuário, função ou pela regra ABAC `owner`.
